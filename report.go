package main

import (
	"fmt"
	"strings"
	"time"
)

// Section holds tabular data for output.
type Section struct {
	Name    string
	Title   string
	Headers []string
	Rows    [][]string
}

// ReportData is the format-agnostic output passed to formatters.
type ReportData struct {
	Metadata map[string]string
	Sections []Section
	Combined Section
}

// GenerateRequest is the input for report generation.
type GenerateRequest struct {
	Name       string        `json:"name"`
	DataSource string        `json:"dataSource"`
	AppID      string        `json:"appId,omitempty"`
	AppIDs     []string      `json:"appIds,omitempty"` // multi-app support
	Columns    []string      `json:"columns,omitempty"`
	Filters    []FilterValue `json:"filters,omitempty"`
	SortBy     string        `json:"sortBy,omitempty"`
	SortDesc   bool          `json:"sortDesc,omitempty"`
	Format     string        `json:"format"`
}

// resolveAppIDs returns the list of app IDs to query.
// Handles: single appId, multiple appIds, or "all".
func (req *GenerateRequest) resolveAppIDs(apps []appJSON) []string {
	if len(req.AppIDs) > 0 {
		if len(req.AppIDs) == 1 && req.AppIDs[0] == "all" {
			ids := make([]string, len(apps))
			for i, a := range apps {
				ids[i] = a.ID
			}
			return ids
		}
		return req.AppIDs
	}
	if req.AppID != "" {
		return []string{req.AppID}
	}
	return nil
}

// ExecuteReport fetches, filters, sorts, projects, and formats a report.
func ExecuteReport(client *C1Client, req GenerateRequest, apps []appJSON) (*ReportData, error) {
	ds := getDataSource(req.DataSource)
	if ds == nil {
		return nil, fmt.Errorf("unknown data source: %s", req.DataSource)
	}

	appIDs := req.resolveAppIDs(apps)
	if ds.RequiresApp && len(appIDs) == 0 {
		return nil, fmt.Errorf("data source %s requires an app selection", req.DataSource)
	}

	// Fetch — multi-app: fetch each app and merge rows
	var rows []map[string]string
	var warnings []string
	if ds.RequiresApp && len(appIDs) > 0 {
		for _, aid := range appIDs {
			r, err := ds.Fetch(client, aid, 0)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Skipped app %s: %v", aid, err))
				continue
			}
			rows = append(rows, r...)
		}
		if len(rows) == 0 && len(warnings) > 0 {
			return nil, fmt.Errorf("all apps failed to fetch data: %s", warnings[0])
		}
	} else {
		var err error
		rows, err = ds.Fetch(client, "", 0)
		if err != nil {
			return nil, fmt.Errorf("fetching data: %w", err)
		}
	}

	// Filter
	rows = applyFilters(rows, req.Filters)

	// Sort
	applySorting(rows, req.SortBy, req.SortDesc)

	// Determine columns
	columns := req.Columns
	if len(columns) == 0 {
		for _, c := range ds.Columns {
			columns = append(columns, c.ID)
		}
	}

	// Project
	headers := columnLabels(ds, columns)
	projected := projectColumns(rows, columns)

	now := time.Now().UTC()
	section := Section{
		Name:    ds.ID,
		Title:   ds.Label,
		Headers: headers,
		Rows:    projected,
	}

	metadata := map[string]string{
		"Report":       req.Name,
		"Data Source":  ds.Label,
		"Generated At": now.Format("2006-01-02 15:04:05 UTC"),
		"Total Rows":   fmt.Sprintf("%d", len(projected)),
		"Tenant":       client.tenant,
	}
	if len(appIDs) == 1 {
		metadata["App ID"] = appIDs[0]
		// Resolve app name
		for _, a := range apps {
			if a.ID == appIDs[0] {
				metadata["App"] = a.DisplayName
				break
			}
		}
	} else if len(appIDs) > 1 {
		metadata["Apps"] = fmt.Sprintf("%d apps", len(appIDs))
	}
	if len(warnings) > 0 {
		metadata["Warnings"] = fmt.Sprintf("%d apps had errors and were skipped", len(warnings))
	}

	// Compute summary stats
	summary := computeSummary(ds, columns, rows)
	for k, v := range summary {
		metadata[k] = v
	}

	return &ReportData{
		Metadata: metadata,
		Sections: []Section{section},
		Combined: section,
	}, nil
}

// computeSummary generates aggregate stats for key columns.
func computeSummary(ds *DataSource, columns []string, rows []map[string]string) map[string]string {
	stats := map[string]string{}
	if len(rows) == 0 {
		return stats
	}

	// Find enum columns and count their values
	colDefs := map[string]ColumnDef{}
	for _, c := range ds.Columns {
		colDefs[c.ID] = c
	}

	for _, colID := range columns {
		def, ok := colDefs[colID]
		if !ok {
			continue
		}

		switch def.Type {
		case "enum":
			counts := map[string]int{}
			for _, r := range rows {
				v := r[colID]
				if v != "" {
					counts[v]++
				}
			}
			if len(counts) > 0 && len(counts) <= 10 {
				parts := []string{}
				for val, count := range counts {
					parts = append(parts, fmt.Sprintf("%s: %d", val, count))
				}
				stats["Summary: "+def.Label] = strings.Join(parts, ", ")
			}
		case "boolean":
			yes, no := 0, 0
			for _, r := range rows {
				if r[colID] == "Yes" {
					yes++
				} else {
					no++
				}
			}
			stats["Summary: "+def.Label] = fmt.Sprintf("Yes: %d, No: %d", yes, no)
		}
	}

	return stats
}

// PreviewReport fetches a small sample for UI preview.
func PreviewReport(client *C1Client, req GenerateRequest, apps []appJSON) ([]string, []map[string]string, int, error) {
	ds := getDataSource(req.DataSource)
	if ds == nil {
		return nil, nil, 0, fmt.Errorf("unknown data source: %s", req.DataSource)
	}

	// Resolve app ID for preview — use first app if "all" selected
	appID := req.AppID
	if len(req.AppIDs) > 0 {
		if req.AppIDs[0] == "all" {
			if len(apps) > 0 {
				appID = apps[0].ID
			}
		} else {
			appID = req.AppIDs[0]
		}
	}

	limit := 50
	rows, err := ds.Fetch(client, appID, limit)
	if err != nil {
		return nil, nil, 0, err
	}

	totalEstimate := len(rows)
	rows = applyFilters(rows, req.Filters)
	applySorting(rows, req.SortBy, req.SortDesc)

	columns := req.Columns
	if len(columns) == 0 {
		for _, c := range ds.Columns {
			columns = append(columns, c.ID)
		}
	}

	headers := columnLabels(ds, columns)

	maxRows := 20
	if len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	preview := make([]map[string]string, len(rows))
	for i, row := range rows {
		m := make(map[string]string)
		for j, col := range columns {
			m[headers[j]] = row[col]
		}
		preview[i] = m
	}

	return headers, preview, totalEstimate, nil
}
