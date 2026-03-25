package main

func appsDataSource() *DataSource {
	return &DataSource{
		ID:          "apps",
		Label:       "Applications",
		Description: "All applications connected to ConductorOne",
		RequiresApp: false,
		Columns: []ColumnDef{
			{ID: "id", Label: "App ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "description", Label: "Description", Type: "string"},
			{ID: "userCount", Label: "User Count", Type: "number"},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
		},
		Fetch: fetchApps,
	}
}

func fetchApps(client *C1Client, _ string, _ int) ([]map[string]string, error) {
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
		rows = append(rows, map[string]string{
			"id":          app.ID,
			"displayName": app.DisplayName,
			"description": strVal(raw, "description"),
			"userCount":   strVal(raw, "userCount"),
			"createdAt":   strVal(raw, "createdAt"),
			"updatedAt":   strVal(raw, "updatedAt"),
		})
	}
	return rows, nil
}
