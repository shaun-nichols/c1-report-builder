package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFS embed.FS

type server struct {
	mu           sync.Mutex
	client       *C1Client
	apps         []appJSON
	templates    *TemplateStore
	lastActivity time.Time
	history      []reportHistoryEntry
	// Cached full report data for the viewer
	viewCache    *viewCacheEntry
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

const sessionTimeout = 30 * time.Minute

type appJSON struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	UserCount   any    `json:"userCount"`
}

func startWebServer() {
	initDataSources()

	srv := &server{
		templates: NewTemplateStore("templates"),
	}

	mux := http.NewServeMux()

	webContent, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, map[string]string{"version": Version})
	})
	mux.HandleFunc("/api/connect", srv.handleConnect)
	mux.HandleFunc("/api/datasources", srv.withSession(srv.handleDataSources))
	mux.HandleFunc("/api/preview", srv.withSession(srv.handlePreview))
	mux.HandleFunc("/api/view", srv.withSession(srv.handleView))
	mux.HandleFunc("/api/generate", srv.withSession(srv.handleGenerate))
	mux.HandleFunc("/api/templates/import", srv.withSession(srv.handleTemplateImport))
	mux.HandleFunc("/api/templates", srv.handleTemplates) // list doesn't need session
	mux.HandleFunc("/api/templates/", srv.handleTemplateByID)
	mux.HandleFunc("/api/history", srv.handleHistory)
	mux.HandleFunc("/api/cleanup", srv.handleCleanup)
	mux.HandleFunc("/api/download/", srv.handleDownload)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
		os.Exit(1)
	}
	addr := listener.Addr().String()
	url := "http://" + addr

	fmt.Printf("ConductorOne Report Builder\n")
	fmt.Printf("Open your browser to: %s\n", url)
	fmt.Printf("Press Ctrl+C to stop.\n\n")

	openBrowser(url)

	if err := http.Serve(listener, mux); err != nil {
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

// withSession checks session is active and refreshes the timeout.
func (s *server) withSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		client := s.client
		expired := !s.lastActivity.IsZero() && time.Since(s.lastActivity) > sessionTimeout
		if client != nil && !expired {
			s.lastActivity = time.Now()
		}
		s.mu.Unlock()

		if client == nil || expired {
			if expired {
				s.mu.Lock()
				s.client = nil
				s.apps = nil
				s.mu.Unlock()
			}
			jsonError(w, "Session expired — please reconnect", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// GET /api/history
func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jsonResp(w, s.history)
}

// POST /api/cleanup — delete output files older than 1 hour
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

// POST /api/connect
func (s *server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	client, err := NewC1Client(req.ClientID, req.ClientSecret)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	appViews, err := client.ListApps()
	if err != nil {
		jsonError(w, "Could not connect: "+err.Error(), http.StatusUnauthorized)
		return
	}

	apps := make([]appJSON, 0, len(appViews))
	for _, av := range appViews {
		app := av.App()
		apps = append(apps, appJSON{ID: app.ID, DisplayName: app.DisplayName, UserCount: app.UserCount})
	}

	s.mu.Lock()
	s.client = client
	s.apps = apps
	s.lastActivity = time.Now()
	s.history = nil
	s.mu.Unlock()

	jsonResp(w, map[string]any{"apps": apps})
}

// GET /api/datasources
func (s *server) handleDataSources(w http.ResponseWriter, r *http.Request) {
	jsonResp(w, allDataSources())
}

// POST /api/preview
func (s *server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		jsonError(w, "Not connected", http.StatusUnauthorized)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	apps := s.apps
	s.mu.Unlock()

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

// POST /api/view — fetch full report data for browser viewing
// Query params: page (1-based), pageSize (default 100), search (optional text filter)
// First call with POST body loads data. Subsequent GET calls paginate the cached data.
func (s *server) handleView(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Load full data
		s.mu.Lock()
		client := s.client
		apps := s.apps
		s.mu.Unlock()
		if client == nil {
			jsonError(w, "Not connected", http.StatusUnauthorized)
			return
		}

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

		setLoading := func(msg string) {
			// server-side progress not visible to client, just log
			fmt.Println("  " + msg)
		}

		// Fetch all data
		appIDs := req.resolveAppIDs(apps)
		var rows []map[string]string
		if ds.RequiresApp && len(appIDs) > 0 {
			setLoading("Fetching data from " + fmt.Sprintf("%d apps", len(appIDs)))
			for _, aid := range appIDs {
				r, err := ds.Fetch(client, aid, 0)
				if err != nil {
					continue
				}
				rows = append(rows, r...)
			}
		} else {
			setLoading("Fetching data...")
			var err error
			rows, err = ds.Fetch(client, "", 0)
			if err != nil {
				jsonError(w, "Fetch failed: "+err.Error(), http.StatusInternalServerError)
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

		// Project to maps with header labels as keys
		projected := make([]map[string]string, len(rows))
		for i, row := range rows {
			m := make(map[string]string, len(columns))
			for j, col := range columns {
				m[headers[j]] = row[col]
			}
			projected[i] = m
		}

		// Cache
		s.mu.Lock()
		s.viewCache = &viewCacheEntry{
			Headers:  headers,
			Rows:     projected,
			Total:    len(projected),
			Metadata: map[string]string{"Total Rows": fmt.Sprintf("%d", len(projected))},
		}
		s.mu.Unlock()
	}

	// Return a page from the cache
	s.mu.Lock()
	cache := s.viewCache
	s.mu.Unlock()

	if cache == nil {
		jsonError(w, "No report loaded. Submit a POST request first.", http.StatusBadRequest)
		return
	}

	// Parse pagination params
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

	// Search filter
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

	pageRows := rows[start:end]

	jsonResp(w, map[string]any{
		"headers":    cache.Headers,
		"rows":       pageRows,
		"total":      total,
		"page":       page,
		"pageSize":   pageSize,
		"totalPages": totalPages,
		"metadata":   cache.Metadata,
	})
}

// POST /api/generate
func (s *server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		jsonError(w, "Not connected", http.StatusUnauthorized)
		return
	}

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

	s.mu.Lock()
	apps := s.apps
	s.mu.Unlock()

	data, err := ExecuteReport(client, req, apps)
	if err != nil {
		jsonError(w, "Report failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll("output", 0o755); err != nil {
		jsonError(w, "Error creating output dir", http.StatusInternalServerError)
		return
	}

	safeName := sanitizeFilename(req.Name)
	ts := data.Metadata["Generated At"]
	ts = strings.ReplaceAll(ts, " ", "_")
	ts = strings.ReplaceAll(ts, ":", "")
	baseName := fmt.Sprintf("c1_%s_%s", safeName, ts)

	files, digest, err := writeReport(data, "output", baseName, req.Format)
	if err != nil {
		jsonError(w, "Write failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Record in history
	s.mu.Lock()
	s.history = append([]reportHistoryEntry{{
		Name:      req.Name,
		Format:    req.Format,
		Rows:      data.Metadata["Total Rows"],
		Files:     files,
		Hash:      digest,
		Timestamp: data.Metadata["Generated At"],
	}}, s.history...) // prepend (newest first)
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

// GET /api/templates — list all
// POST /api/templates — save new
func (s *server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResp(w, s.templates.List())
	case http.MethodPost:
		var t ReportTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.templates.Save(t); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, t)
	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// /api/templates/{id} — GET, PUT, DELETE
// /api/templates/{id}/clone — POST
func (s *server) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	// Clone endpoint
	if len(parts) == 2 && parts[1] == "clone" && r.Method == http.MethodPost {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		clone, err := s.templates.Clone(id, req.Name)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
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
		var t ReportTemplate
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		t.ID = id
		if err := s.templates.Save(t); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResp(w, t)

	case http.MethodDelete:
		if err := s.templates.Delete(id); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		jsonResp(w, map[string]string{"status": "deleted"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/templates/import — import a template from JSON
func (s *server) handleTemplateImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var t ReportTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonError(w, "Invalid template JSON", http.StatusBadRequest)
		return
	}
	t.Builtin = false
	t.ID = "" // generate new ID
	if err := s.templates.Save(t); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResp(w, t)
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/download/")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	path := filepath.Join("output", name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, path)
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
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
