package main

import (
	"sort"
	"strconv"
	"strings"
)

// ColumnDef describes a column available in a data source.
type ColumnDef struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"` // "string", "enum", "date", "number", "boolean"
	Options     []string `json:"options,omitempty"`
}

// DataSource defines a queryable data type from ConductorOne.
type DataSource struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	RequiresApp bool        `json:"requiresApp"`
	Columns     []ColumnDef `json:"columns"`
	Fetch       func(client *C1Client, appID string, limit int) ([]map[string]string, error) `json:"-"`
}

// FetchParams controls how data is fetched and filtered.
type FetchParams struct {
	AppID   string        `json:"appId,omitempty"`
	Filters []FilterValue `json:"filters,omitempty"`
	Limit   int           `json:"limit,omitempty"`
}

// FilterValue represents a single filter condition.
type FilterValue struct {
	ColumnID string `json:"columnId"`
	Operator string `json:"operator"` // eq, neq, contains, not_contains, empty, not_empty, gt, lt, before, after
	Value    string `json:"value"`
}

// dataSources is the global registry.
var dataSources map[string]*DataSource

func initDataSources() {
	dataSources = map[string]*DataSource{
		"apps":         appsDataSource(),
		"app_overview": appOverviewDataSource(),
		"app_users":    appUsersDataSource(),
		"entitlements": entitlementsDataSource(),
		"grants":       grantsDataSource(),
		"connectors":   connectorsDataSource(),
		"tasks":        tasksDataSource(),
		"system_log":   systemLogDataSource(),
		"grant_feed":   grantFeedDataSource(),
		"past_grants":  pastGrantsDataSource(),
		"app_owners":   appOwnersDataSource(),
	}
}

func getDataSource(id string) *DataSource {
	return dataSources[id]
}

func allDataSources() []*DataSource {
	var list []*DataSource
	for _, ds := range dataSources {
		list = append(list, ds)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Label < list[j].Label })
	return list
}

// columnLabels returns the human-readable labels for a set of column IDs.
func columnLabels(ds *DataSource, columnIDs []string) []string {
	labelMap := map[string]string{}
	for _, c := range ds.Columns {
		labelMap[c.ID] = c.Label
	}
	labels := make([]string, len(columnIDs))
	for i, id := range columnIDs {
		if l, ok := labelMap[id]; ok {
			labels[i] = l
		} else {
			labels[i] = id
		}
	}
	return labels
}

// projectColumns selects and orders columns from rows.
func projectColumns(rows []map[string]string, columnIDs []string) [][]string {
	result := make([][]string, len(rows))
	for i, row := range rows {
		projected := make([]string, len(columnIDs))
		for j, col := range columnIDs {
			projected[j] = row[col]
		}
		result[i] = projected
	}
	return result
}

// applyFilters filters rows based on the given filter conditions.
func applyFilters(rows []map[string]string, filters []FilterValue) []map[string]string {
	if len(filters) == 0 {
		return rows
	}
	var result []map[string]string
	for _, row := range rows {
		if matchesAllFilters(row, filters) {
			result = append(result, row)
		}
	}
	return result
}

func matchesAllFilters(row map[string]string, filters []FilterValue) bool {
	for _, f := range filters {
		val := row[f.ColumnID]
		switch f.Operator {
		case "eq":
			if !strings.EqualFold(val, f.Value) {
				return false
			}
		case "neq":
			if strings.EqualFold(val, f.Value) {
				return false
			}
		case "contains":
			if !strings.Contains(strings.ToLower(val), strings.ToLower(f.Value)) {
				return false
			}
		case "not_contains":
			if strings.Contains(strings.ToLower(val), strings.ToLower(f.Value)) {
				return false
			}
		case "empty":
			if val != "" {
				return false
			}
		case "not_empty":
			if val == "" {
				return false
			}
		case "gt":
			a, _ := strconv.ParseFloat(val, 64)
			b, _ := strconv.ParseFloat(f.Value, 64)
			if a <= b {
				return false
			}
		case "lt":
			a, _ := strconv.ParseFloat(val, 64)
			b, _ := strconv.ParseFloat(f.Value, 64)
			if a >= b {
				return false
			}
		case "before":
			if val >= f.Value {
				return false
			}
		case "after":
			if val <= f.Value {
				return false
			}
		}
	}
	return true
}

// applySorting sorts rows by the given column.
func applySorting(rows []map[string]string, sortBy string, sortDesc bool) {
	if sortBy == "" {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a := rows[i][sortBy]
		b := rows[j][sortBy]
		// Try numeric sort first
		an, errA := strconv.ParseFloat(a, 64)
		bn, errB := strconv.ParseFloat(b, 64)
		if errA == nil && errB == nil {
			if sortDesc {
				return an > bn
			}
			return an < bn
		}
		// Fall back to string sort
		cmp := strings.Compare(strings.ToLower(a), strings.ToLower(b))
		if sortDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}
