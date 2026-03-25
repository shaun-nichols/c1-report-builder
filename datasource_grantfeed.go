package main

import "encoding/json"

func grantFeedDataSource() *DataSource {
	return &DataSource{
		ID:          "grant_feed",
		Label:       "Grant Change Feed",
		Description: "Audit trail of all access grant additions and removals",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "date", Label: "Date", Type: "date"},
			{ID: "eventType", Label: "Event Type", Type: "enum", Options: []string{"GRANT_EVENT_TYPE_ADDED", "GRANT_EVENT_TYPE_REMOVED"}},
			{ID: "appId", Label: "App ID", Type: "string"},
			{ID: "appUserId", Label: "App User ID", Type: "string"},
			{ID: "appEntitlementId", Label: "Entitlement ID", Type: "string"},
			{ID: "ticketId", Label: "Ticket ID", Type: "string"},
		},
		Fetch: fetchGrantFeed,
	}
}

func fetchGrantFeed(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	body := map[string]any{}
	if appID != "" {
		body["appRefs"] = []map[string]string{{"id": appID}}
	}

	raw, err := client.SearchGrantFeed(body, limit)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, r := range raw {
		var event map[string]any
		if err := json.Unmarshal(r, &event); err != nil {
			continue
		}
		rows = append(rows, map[string]string{
			"date":             strVal(event, "date"),
			"eventType":        strVal(event, "eventType"),
			"appId":            strVal(event, "appId"),
			"appUserId":        strVal(event, "appUserId"),
			"appEntitlementId": strVal(event, "appEntitlementId"),
			"ticketId":         strVal(event, "ticketId"),
		})
	}
	return rows, nil
}
