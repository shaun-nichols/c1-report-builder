package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ReportTemplate defines a saved report configuration.
type ReportTemplate struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category,omitempty"` // "Data Audit", "Governance", "Operations"
	Builtin     bool          `json:"builtin"`
	DataSource  string        `json:"dataSource"`
	Columns     []string      `json:"columns"`
	Filters     []FilterValue `json:"filters,omitempty"`
	SortBy      string        `json:"sortBy,omitempty"`
	SortDesc    bool          `json:"sortDesc,omitempty"`
	Format      string        `json:"format"`
	CreatedAt   string        `json:"createdAt,omitempty"`
	UpdatedAt   string        `json:"updatedAt,omitempty"`
}

// TemplateStore manages builtin and user-saved templates.
type TemplateStore struct {
	dir     string
	builtin map[string]ReportTemplate
	mu      sync.RWMutex
	user    map[string]ReportTemplate
}

func NewTemplateStore(dir string) *TemplateStore {
	ts := &TemplateStore{
		dir:     dir,
		builtin: make(map[string]ReportTemplate),
		user:    make(map[string]ReportTemplate),
	}
	for _, t := range builtinTemplates() {
		ts.builtin[t.ID] = t
	}
	ts.loadUserTemplates()
	return ts
}

func (ts *TemplateStore) List() []ReportTemplate {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	var list []ReportTemplate
	for _, t := range ts.builtin {
		list = append(list, t)
	}
	for _, t := range ts.user {
		list = append(list, t)
	}
	return list
}

func (ts *TemplateStore) Get(id string) (ReportTemplate, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if t, ok := ts.builtin[id]; ok {
		return t, true
	}
	if t, ok := ts.user[id]; ok {
		return t, true
	}
	return ReportTemplate{}, false
}

func (ts *TemplateStore) Save(t ReportTemplate) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if t.ID == "" {
		t.ID = generateID(t.Name)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	t.Builtin = false

	if err := os.MkdirAll(ts.dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(ts.dir, t.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	ts.user[t.ID] = t
	return nil
}

func (ts *TemplateStore) Delete(id string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, ok := ts.builtin[id]; ok {
		return fmt.Errorf("cannot delete builtin template")
	}

	path := filepath.Join(ts.dir, id+".json")
	os.Remove(path)
	delete(ts.user, id)
	return nil
}

func (ts *TemplateStore) Clone(id, newName string) (ReportTemplate, error) {
	t, ok := ts.Get(id)
	if !ok {
		return ReportTemplate{}, fmt.Errorf("template %s not found", id)
	}

	clone := t
	clone.ID = generateID(newName)
	clone.Name = newName
	clone.Builtin = false
	clone.CreatedAt = ""

	if err := ts.Save(clone); err != nil {
		return ReportTemplate{}, err
	}
	return clone, nil
}

func (ts *TemplateStore) loadUserTemplates() {
	entries, err := os.ReadDir(ts.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ts.dir, e.Name()))
		if err != nil {
			continue
		}
		var t ReportTemplate
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		t.Builtin = false
		ts.user[t.ID] = t
	}
}

func generateID(name string) string {
	id := strings.ToLower(name)
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, id)
	// Collapse multiple dashes
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	id = strings.Trim(id, "-")
	if id == "" {
		id = fmt.Sprintf("report-%d", time.Now().UnixMilli())
	}
	return id
}
