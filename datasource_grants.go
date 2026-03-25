package main

func grantsDataSource() *DataSource {
	return &DataSource{
		ID:          "grants",
		Label:       "Grants",
		Description: "Account-entitlement assignments with user and entitlement details",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "appUserId", Label: "App User ID", Type: "string"},
			{ID: "appUserDisplayName", Label: "User Display Name", Type: "string"},
			{ID: "appUserEmail", Label: "User Email", Type: "string"},
			{ID: "appUserUsername", Label: "User Username", Type: "string"},
			{ID: "appUserStatus", Label: "User Status", Type: "enum", Options: []string{"STATUS_ENABLED", "STATUS_DISABLED"}},
			{ID: "appUserType", Label: "User Type", Type: "enum", Options: []string{"APP_USER_TYPE_USER", "APP_USER_TYPE_SERVICE_ACCOUNT", "APP_USER_TYPE_SYSTEM_ACCOUNT"}},
			{ID: "identityUserId", Label: "Identity User ID", Type: "string"},
			{ID: "entitlementId", Label: "Entitlement ID", Type: "string"},
			{ID: "entitlementDisplayName", Label: "Entitlement Name", Type: "string"},
			{ID: "entitlementDescription", Label: "Entitlement Description", Type: "string"},
			{ID: "entitlementAlias", Label: "Entitlement Alias", Type: "string"},
			{ID: "entitlementResourceType", Label: "Resource Type", Type: "string"},
			{ID: "grantedAt", Label: "Granted At", Type: "date"},
			{ID: "expiresAt", Label: "Expires At", Type: "date"},
			{ID: "grantSources", Label: "Grant Sources", Type: "string"},
		},
		Fetch: fetchGrants,
	}
}

func fetchGrants(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	grantsRaw, err := client.SearchGrants(appID, limit)
	if err != nil {
		return nil, err
	}
	grants := parseGrants(grantsRaw)

	var rows []map[string]string
	for _, g := range grants {
		uid := strValGrant(g, "_appUserId")
		if uid == "" {
			continue // skip malformed grant records
		}
		rows = append(rows, map[string]string{
			"appUserId":               uid,
			"appUserDisplayName":      strValGrant(g, "_appUserDisplayName"),
			"appUserEmail":            strValGrant(g, "_appUserEmail"),
			"appUserUsername":         strValGrant(g, "_appUserUsername"),
			"appUserStatus":           strValGrant(g, "_appUserStatus"),
			"appUserType":             strValGrant(g, "_appUserType"),
			"identityUserId":          strValGrant(g, "_identityUserId"),
			"entitlementId":           strValGrant(g, "_entitlementId"),
			"entitlementDisplayName":  strValGrant(g, "_entitlementDisplayName"),
			"entitlementDescription":  strValGrant(g, "_entitlementDescription"),
			"entitlementAlias":        strValGrant(g, "_entitlementAlias"),
			"entitlementResourceType": strValGrant(g, "_entitlementResourceTypeId"),
			"grantedAt":              strVal(g, "createdAt"),
			"expiresAt":              strVal(g, "deprovisionAt"),
			"grantSources":           strVal(g, "grantSources"),
		})
	}
	return rows, nil
}

func strValGrant(g map[string]any, key string) string {
	v, ok := g[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
