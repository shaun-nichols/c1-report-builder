package main

import "encoding/json"

func pastGrantsDataSource() *DataSource {
	return &DataSource{
		ID:          "past_grants",
		Label:       "Past / Revoked Grants",
		Description: "Historical grant periods with granted and revoked timestamps",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "appId", Label: "App ID", Type: "string"},
			{ID: "appUserId", Label: "App User ID", Type: "string"},
			{ID: "appEntitlementId", Label: "Entitlement ID", Type: "string"},
			{ID: "grantedAt", Label: "Granted At", Type: "date"},
			{ID: "revokedAt", Label: "Revoked At", Type: "date"},
		},
		Fetch: fetchPastGrants,
	}
}

func fetchPastGrants(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	raw, err := client.SearchPastGrants(appID, limit)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, r := range raw {
		var view map[string]any
		if err := json.Unmarshal(r, &view); err != nil {
			continue
		}

		// The response wraps in "history" key
		history := mapVal(view, "history")
		if history == nil {
			history = view
		}

		rows = append(rows, map[string]string{
			"appId":            strVal(history, "appId"),
			"appUserId":        strVal(history, "appUserId"),
			"appEntitlementId": strVal(history, "appEntitlementId"),
			"grantedAt":        strVal(history, "grantedAt"),
			"revokedAt":        strVal(history, "revokedAt"),
		})
	}
	return rows, nil
}
