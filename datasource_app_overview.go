package main

import (
	"encoding/json"
	"fmt"
)

func appOverviewDataSource() *DataSource {
	return &DataSource{
		ID:          "app_overview",
		Label:       "App Overview (Enriched)",
		Description: "Applications with owner count, connector sync status, and user count",
		RequiresApp: false,
		Columns: []ColumnDef{
			{ID: "id", Label: "App ID", Type: "string"},
			{ID: "displayName", Label: "App Name", Type: "string"},
			{ID: "description", Label: "Description", Type: "string"},
			{ID: "userCount", Label: "User Count", Type: "number"},
			{ID: "ownerCount", Label: "Owner Count", Type: "number"},
			{ID: "owners", Label: "Owners", Description: "Names of app owners", Type: "string"},
			{ID: "hasOwner", Label: "Has Owner", Type: "boolean"},
			{ID: "syncStatus", Label: "Sync Status", Type: "enum", Options: []string{"SYNC_STATUS_DONE", "SYNC_STATUS_RUNNING", "SYNC_STATUS_ERROR", "SYNC_STATUS_DISABLED", "No Connector"}},
			{ID: "syncCompletedAt", Label: "Last Sync", Type: "date"},
			{ID: "syncError", Label: "Sync Error", Type: "string"},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
		},
		Fetch: fetchAppOverview,
	}
}

func fetchAppOverview(client *C1Client, _ string, _ int) ([]map[string]string, error) {
	appViews, err := client.ListApps()
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, av := range appViews {
		app := av.App()
		raw := av
		if nested, ok := raw["app"].(map[string]any); ok {
			raw = AppView(nested)
		}

		// Fetch owners
		ownerCount := 0
		ownerNames := ""
		hasOwner := "No"
		ownerRaw, err := client.ListAppOwners(app.ID)
		if err == nil && len(ownerRaw) > 0 {
			ownerCount = len(ownerRaw)
			hasOwner = "Yes"
			var names []string
			for _, o := range ownerRaw {
				var u map[string]any
				if err := json.Unmarshal(o, &u); err == nil {
					name := strVal(u, "displayName")
					if name == "" {
						name = strVal(u, "email")
					}
					if name != "" {
						names = append(names, name)
					}
				}
			}
			for i, n := range names {
				if i > 0 {
					ownerNames += "; "
				}
				ownerNames += n
			}
		}

		// Fetch connector sync status
		syncStatus := "No Connector"
		syncCompletedAt := ""
		syncError := ""
		connRaw, err := client.ListConnectors(app.ID)
		if err == nil && len(connRaw) > 0 {
			var conn map[string]any
			if err := json.Unmarshal(connRaw[0], &conn); err == nil {
				if status, ok := conn["status"].(map[string]any); ok {
					syncStatus = strVal(status, "status")
					syncCompletedAt = strVal(status, "completedAt")
					syncError = strVal(status, "lastError")
				}
			}
		}

		rows = append(rows, map[string]string{
			"id":              app.ID,
			"displayName":     app.DisplayName,
			"description":     strVal(raw, "description"),
			"userCount":       strVal(raw, "userCount"),
			"ownerCount":      fmt.Sprintf("%d", ownerCount),
			"owners":          ownerNames,
			"hasOwner":        hasOwner,
			"syncStatus":      syncStatus,
			"syncCompletedAt": syncCompletedAt,
			"syncError":       syncError,
			"createdAt":       strVal(raw, "createdAt"),
			"updatedAt":       strVal(raw, "updatedAt"),
		})
	}
	return rows, nil
}
