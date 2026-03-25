package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFS embed.FS

const (
	sessionTimeout = 30 * time.Minute
	maxRequestBody = 1 << 20 // 1MB
)

type server struct {
	mu           sync.Mutex
	client       *C1Client
	apps         []appJSON
	templates    *TemplateStore
	lastActivity time.Time
	history      []reportHistoryEntry
	viewCache    *viewCacheEntry
	sessionToken string // random token required on all authenticated requests
	listenAddr   string // the actual address we're listening on
}

type viewCacheEntry struct {
	Headers  []string
	Rows     []map[string]string
	Metadata map[string]string
	Total    int
}

type reportHistoryEntry struct {
	Name      string   `json:"name"`
	Format    string   `json:"format"`
	Rows      string   `json:"rows"`
	Files     []string `json:"files"`
	Hash      string   `json:"hash"`
	Timestamp string   `json:"timestamp"`
}

type appJSON struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	UserCount   any    `json:"userCount"`
}

// Safe template ID pattern
var safeIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func startWebServer() {
	initDataSources()

	srv := &server{
		templates: NewTemplateStore("templates"),
	}

	mux := http.NewServeMux()

	webContent, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Public endpoints (no session required)
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, map[string]string{"version": Version})
	})
	mux.HandleFunc("/api/credentials", srv.handleCredentials)
	mux.HandleFunc("/api/connect", srv.handleConnect)

	// Session-protected endpoints
	mux.HandleFunc("/api/datasources", srv.withSession(srv.handleDataSources))
	mux.HandleFunc("/api/preview", srv.withSession(srv.handlePreview))
	mux.HandleFunc("/api/view", srv.withSession(srv.handleView))
	mux.HandleFunc("/api/generate", srv.withSession(srv.handleGenerate))
	mux.HandleFunc("/api/templates/import", srv.withSession(srv.handleTemplateImport))
	mux.HandleFunc("/api/templates", srv.withSession(srv.handleTemplates))
	mux.HandleFunc("/api/templates/", srv.withSession(srv.handleTemplateByID))
	mux.HandleFunc("/api/history", srv.withSession(srv.handleHistory))
	mux.HandleFunc("/api/cleanup", srv.withSession(srv.handleCleanup))
	mux.HandleFunc("/api/download/", srv.withSession(srv.handleDownload))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
	srv.listenAddr = listener.Addr().String()
	url := "http://" + srv.listenAddr

	fmt.Printf("ConductorOne Report Builder\n")
	fmt.Printf("Open your browser to: %s\n", url)
	fmt.Printf("Press Ctrl+C to stop.\n\n")

	openBrowser(url)

	// Wrap mux with security middleware
	handler := srv.securityHeaders(srv.csrfProtection(mux))

	if err := http.Serve(listener, handler); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// --- Security middleware ---

// securityHeaders adds standard security response headers.
func (s *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// csrfProtection validates Origin header on mutating requests.
func (s *server) csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only check mutating methods on API endpoints
		if strings.HasPrefix(r.URL.Path, "/api/") &&
			(r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete) {

			origin := r.Header.Get("Origin")
			// Allow requests with no Origin (curl, non-browser clients)
			if origin != "" {
				allowed := "http://" + s.listenAddr
				if origin != allowed {
					jsonError(w, "Forbidden: invalid origin", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// withSession checks session token and timeout.
func (s *server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		token := s.sessionToken
		client := s.client
		expired := !s.lastActivity.IsZero() && time.Since(s.lastActivity) > sessionTimeout
		if client != nil && !expired {
			s.lastActivity = time.Now()
		}
		s.mu.Unlock()

		// Check session token
		reqToken := r.Header.Get("X-Session-Token")
		if token == "" || reqToken != token {
			// Also check query param for download links (can't set headers on <a href>)
			if r.URL.Query().Get("token") != token || token == "" {
				jsonError(w, "Session expired — please reconnect", http.StatusUnauthorized)
				return
			}
		}

		if client == nil || expired {
			if expired {
				s.mu.Lock()
				s.client = nil
				s.apps = nil
				s.sessionToken = ""
				s.mu.Unlock()
			}
			jsonError(w, "Session expired — please reconnect", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// limitBody wraps a request body with a size limit.
func limitBody(r *http.Request, w http.ResponseWriter) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
}

// generateSessionToken creates a cryptographically random token.
func generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Handlers ---

func (s *server) handleCredentials(w http.ResponseWriter, r *http.Request) {
	cs := newCredentialStore()

	switch r.Method {
	case http.MethodGet:
		available := cs.isAvailable()
		hasSaved := false
		if available {
			id, secret, _ := cs.load()
			hasSaved = id != "" && secret != ""
		}
		jsonResp(w, map[string]any{
			"keyringAvailable":    available,
			"keyringName":         cs.platformName(),
			"hasSavedCredentials": hasSaved,
		})

	case http.MethodDelete:
		if err := cs.clear(); err != nil {
			jsonError(w, "Failed to clear saved credentials", http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]string{"status": "cleared"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(r, w)

	var req struct {
		ClientID      string `json:"clientId"`
		ClientSecret  string `json:"clientSecret"`
		SaveToKeyring bool   `json:"saveToKeyring"`
		UseKeyring    bool   `json:"useKeyring"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.UseKeyring {
		cs := newCredentialStore()
		id, secret, err := cs.load()
		if err != nil || id == "" || secret == "" {
			jsonError(w, "No saved credentials found", http.StatusBadRequest)
			return
		}
		req.ClientID = id
		req.ClientSecret = secret
	}

	if req.ClientID == "" || req.ClientSecret == "" {
		jsonError(w, "Client ID and Client Secret are required", http.StatusBadRequest)
		return
	}

	client, err := NewC1Client(req.ClientID, req.ClientSecret)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	appViews, err := client.ListApps()
	if err != nil {
		// Generic error — don't leak auth details
		jsonError(w, "Authentication failed — check your Client ID and Client Secret", http.StatusUnauthorized)
		return
	}

	if req.SaveToKeyring {
		cs := newCredentialStore()
		if err := cs.save(req.ClientID, req.ClientSecret); err != nil {
			fmt.Printf("Warning: failed to save to keyring: %v\n", err)
		}
	}

	apps := make([]appJSON, 0, len(appViews))
	for _, av := range appViews {
		app := av.App()
		apps = append(apps, appJSON{ID: app.ID, DisplayName: app.DisplayName, UserCount: app.UserCount})
	}

	token := generateSessionToken()

	s.mu.Lock()
	s.client = client
	s.apps = apps
	s.lastActivity = time.Now()
	s.history = nil
	s.sessionToken = token
	s.viewCache = nil
	s.mu.Unlock()

	jsonResp(w, map[string]any{"apps": apps, "sessionToken": token})
}

func (s *server) handleDataSources(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, allDataSources())
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jsonResp(w, s.history)
}

func (s *server) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := os.ReadDir("output")
	if err != nil {
		jsonResp(w, map[string]any{"deleted": 0})
		return
	}
	deleted := 0
	cutoff := time.Now().Add(-1 * time.Hour)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join("output", e.Name()))
			deleted++
		}
	}
	jsonResp(w, map[string]any{"deleted": deleted})
}

func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(r, w)

	s.mu.Lock()
	client := s.client
	apps := s.apps
	s.mu.Unlock()

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	headers, rows, total, err := PreviewReport(client, req, apps)
	if err != nil {
		jsonError(w, "Preview failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResp(w, map[string]any{
		"headers":   headers,
		"rows":      rows,
		"total":     total,
		"truncated": total > len(rows),
	})
}

func (s *server) handleView(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		limitBody(r, w)
		s.mu.Lock()
		client := s.client
		apps := s.apps
		s.mu.Unlock()

		var req GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		ds := getDataSource(req.DataSource)
		if ds == nil {
			jsonError(w, "Unknown data source", http.StatusBadRequest)
			return
		}

		appIDs := req.resolveAppIDs(apps)
		var rows []map[string]string
		if ds.RequiresApp && len(appIDs) > 0 {
			for _, aid := range appIDs {
				r, err := ds.Fetch(client, aid, 0)
				if err != nil {
					continue
				}
				rows = append(rows, r...)
			}
		} else {
			var err error
			rows, err = ds.Fetch(client, "", 0)
			if err != nil {
				jsonError(w, "Fetch failed", http.StatusInternalServerError)
				return
			}
		}

		rows = applyFilters(rows, req.Filters)
		applySorting(rows, req.SortBy, req.SortDesc)

		columns := req.Columns
		if len(columns) == 0 {
			for _, c := range ds.Columns {
				columns = append(columns, c.ID)
			}
		}
		headers := columnLabels(ds, columns)

		projected := make([]map[string]string, len(rows))
		for i, row := range rows {
			m := make(map[string]string, len(columns))
			for j, col := range columns {
				m[headers[j]] = row[col]
			}
			projected[i] = m
		}

		s.mu.Lock()
		s.viewCache = &viewCacheEntry{
			Headers:  headers,
			Rows:     projected,
			Total:    len(projected),
			Metadata: map[string]string{"Total Rows": fmt.Sprintf("%d", len(projected))},
		}
		s.mu.Unlock()
	}

	s.mu.Lock()
	cache := s.viewCache
	s.mu.Unlock()

	if cache == nil {
		jsonError(w, "No report loaded", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	page := 1
	pageSize := 100
	if v := q.Get("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := q.Get("pageSize"); v != "" {
		fmt.Sscanf(v, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 10 {
		pageSize = 10
	}
	if pageSize > 500 {
		pageSize = 500
	}

	search := strings.ToLower(q.Get("search"))
	rows := cache.Rows
	if search != "" {
		var filtered []map[string]string
		for _, row := range rows {
			for _, val := range row {
				if strings.Contains(strings.ToLower(val), search) {
					filtered = append(filtered, row)
					break
				}
			}
		}
		rows = filtered
	}

	total := len(rows)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}

	jsonResp(w, map[string]any{
		"headers":    cache.Headers,
		"rows":       rows[start:end],
		"total":      total,
		"page":       page,
		"pageSize":   pageSize,
		"totalPages": totalPages,
		"metadata":   cache.Metadata,
	})
}

func (s *server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(r, w)

	s.mu.Lock()
	client := s.client
	apps := s.apps
	s.mu.Unlock()

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.Format == "" {
		req.Format = "csv"
	}
	if req.Name == "" {
		req.Name = "Custom Report"
	}

	data, err := ExecuteReport(client, req, apps)
	if err != nil {
		jsonError(w, "Report generation failed", http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll("output", 0o700); err != nil {
		jsonError(w, "Could not create output directory", http.StatusInternalServerError)
		return
	}

	safeName := sanitizeFilename(req.Name)
	ts := data.Metadata["Generated At"]
	ts = strings.ReplaceAll(ts, " ", "_")
	ts = strings.ReplaceAll(ts, ":", "")
	baseName := fmt.Sprintf("c1_%s_%s", safeName, ts)

	files, digest, err := writeReport(data, "output", baseName, req.Format)
	if err != nil {
		jsonError(w, "Failed to write report", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.history = append([]reportHistoryEntry{{
		Name:      req.Name,
		Format:    req.Format,
		Rows:      data.Metadata["Total Rows"],
		Files:     files,
		Hash:      digest,
		Timestamp: data.Metadata["Generated At"],
	}}, s.history...)
	if len(s.history) > 50 {
		s.history = s.history[:50]
	}
	s.mu.Unlock()

	jsonResp(w, map[string]any{
		"files":    files,
		"hash":     digest,
		"metadata": data.Metadata,
	})
}

func (s *server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, s.templates.List())
	case http.MethodPost:
		limitBody(r, w)
		var t ReportTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.templates.Save(t); err != nil {
			jsonError(w, "Failed to save template", http.StatusInternalServerError)
			return
		}
		jsonResp(w, t)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	// Validate template ID — prevent path traversal
	if !safeIDPattern.MatchString(id) {
		jsonError(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	// Clone endpoint
	if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
		limitBody(r, w)
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		clone, err := s.templates.Clone(id, req.Name)
		if err != nil {
			jsonError(w, "Failed to clone template", http.StatusInternalServerError)
			return
		}
		jsonResp(w, clone)
		return
	}

	switch r.Method {
	case http.MethodGet:
		t, ok := s.templates.Get(id)
		if !ok {
			jsonError(w, "Template not found", http.StatusNotFound)
			return
		}
		jsonResp(w, t)

	case http.MethodPut:
		limitBody(r, w)
		var t ReportTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		t.ID = id
		if err := s.templates.Save(t); err != nil {
			jsonError(w, "Failed to save template", http.StatusInternalServerError)
			return
		}
		jsonResp(w, t)

	case http.MethodDelete:
		if err := s.templates.Delete(id); err != nil {
			jsonError(w, "Cannot delete this template", http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]string{"status": "deleted"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleTemplateImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitBody(r, w)
	var t ReportTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonError(w, "Invalid template JSON", http.StatusBadRequest)
		return
	}
	t.Builtin = false
	t.ID = ""
	if err := s.templates.Save(t); err != nil {
		jsonError(w, "Failed to import template", http.StatusInternalServerError)
		return
	}
	jsonResp(w, t)
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/download/")
	// Strict filename validation
	if name == "" || strings.ContainsAny(name, "../\\\"") || !sanitizedFilename(name) {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	path := filepath.Join("output", name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(name)))
	http.ServeFile(w, r, path)
}

// sanitizedFilename checks if a filename contains only safe characters.
func sanitizedFilename(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return len(name) > 0 && len(name) < 256
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_.")
}

func jsonResp(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
