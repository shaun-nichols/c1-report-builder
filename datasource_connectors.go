package main

import "encoding/json"

func connectorsDataSource() *DataSource {
	return &DataSource{
		ID:          "connectors",
		Label:       "Connectors",
		Description: "Connector sync status and health for an application",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "id", Label: "Connector ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "syncStatus", Label: "Sync Status", Type: "enum", Options: []string{"SYNC_STATUS_DONE", "SYNC_STATUS_RUNNING", "SYNC_STATUS_ERROR", "SYNC_STATUS_DISABLED"}},
			{ID: "syncStartedAt", Label: "Sync Started At", Type: "date"},
			{ID: "syncCompletedAt", Label: "Sync Completed At", Type: "date"},
			{ID: "lastError", Label: "Last Error", Type: "string"},
		},
		Fetch: fetchConnectors,
	}
}

func fetchConnectors(client *C1Client, appID string, _ int) ([]map[string]string, error) {
	items, err := client.ListConnectors(appID)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, raw := range items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		row := map[string]string{
			"id":          strVal(m, "id"),
			"displayName": strVal(m, "displayName"),
		}
		if status, ok := m["status"].(map[string]any); ok {
			row["syncStatus"] = strVal(status, "status")
			row["syncStartedAt"] = strVal(status, "startedAt")
			row["syncCompletedAt"] = strVal(status, "completedAt")
			row["lastError"] = strVal(status, "lastError")
		}
		rows = append(rows, row)
	}
	return rows, nil
}
