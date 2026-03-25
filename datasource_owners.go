package main

import "encoding/json"

func appOwnersDataSource() *DataSource {
	return &DataSource{
		ID:          "app_owners",
		Label:       "App Owners",
		Description: "Users assigned as owners for an application",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "userId", Label: "User ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "email", Label: "Email", Type: "string"},
			{ID: "username", Label: "Username", Type: "string"},
			{ID: "department", Label: "Department", Type: "string"},
			{ID: "jobTitle", Label: "Job Title", Type: "string"},
			{ID: "status", Label: "Status", Type: "enum", Options: []string{"ENABLED", "DISABLED"}},
		},
		Fetch: fetchAppOwners,
	}
}

func fetchAppOwners(client *C1Client, appID string, _ int) ([]map[string]string, error) {
	raw, err := client.ListAppOwners(appID)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, r := range raw {
		var user map[string]any
		if err := json.Unmarshal(r, &user); err != nil {
			continue
		}
		rows = append(rows, map[string]string{
			"userId":      strVal(user, "id"),
			"displayName": strVal(user, "displayName"),
			"email":       strVal(user, "email"),
			"username":    strVal(user, "username"),
			"department":  strVal(user, "department"),
			"jobTitle":    strVal(user, "jobTitle"),
			"status":      strVal(user, "status"),
		})
	}
	return rows, nil
}
