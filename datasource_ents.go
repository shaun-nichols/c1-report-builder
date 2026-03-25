package main

func entitlementsDataSource() *DataSource {
	return &DataSource{
		ID:          "entitlements",
		Label:       "Entitlements",
		Description: "Roles, groups, and permissions within an application",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "id", Label: "Entitlement ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "description", Label: "Description", Type: "string"},
			{ID: "alias", Label: "Alias", Type: "string"},
			{ID: "appResourceTypeId", Label: "Resource Type ID", Type: "string"},
			{ID: "appResourceId", Label: "Resource ID", Type: "string"},
			{ID: "slug", Label: "Slug", Type: "string"},
			{ID: "grantCount", Label: "Grant Count", Type: "number"},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
		},
		Fetch: fetchEntitlements,
	}
}

func fetchEntitlements(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	entsRaw, err := client.ListEntitlements(appID, limit)
	if err != nil {
		return nil, err
	}
	ents := parseItems(entsRaw, "appEntitlement")

	var rows []map[string]string
	for _, e := range ents {
		rows = append(rows, map[string]string{
			"id":                strVal(e, "id"),
			"displayName":      strVal(e, "displayName"),
			"description":      strVal(e, "description"),
			"alias":            strVal(e, "alias"),
			"appResourceTypeId": strVal(e, "appResourceTypeId"),
			"appResourceId":    strVal(e, "appResourceId"),
			"slug":             strVal(e, "slug"),
			"grantCount":       strVal(e, "grantCount"),
			"createdAt":        strVal(e, "createdAt"),
			"updatedAt":        strVal(e, "updatedAt"),
		})
	}
	return rows, nil
}
