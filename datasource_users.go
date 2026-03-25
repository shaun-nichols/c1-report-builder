package main

import "fmt"

func appUsersDataSource() *DataSource {
	return &DataSource{
		ID:          "app_users",
		Label:       "App Users",
		Description: "User accounts within an application",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "id", Label: "App User ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "email", Label: "Email", Type: "string"},
			{ID: "username", Label: "Username", Type: "string"},
			{ID: "identityUserId", Label: "Identity User ID", Type: "string"},
			{ID: "status", Label: "Status", Type: "enum", Options: []string{"STATUS_ENABLED", "STATUS_DISABLED"}},
			{ID: "appUserType", Label: "User Type", Type: "enum", Options: []string{"APP_USER_TYPE_USER", "APP_USER_TYPE_SERVICE_ACCOUNT", "APP_USER_TYPE_SYSTEM_ACCOUNT"}},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
			{ID: "grantCount", Label: "Grant Count", Description: "Number of entitlement grants", Type: "number"},
			{ID: "orphaned", Label: "Orphaned", Description: "No identity mapping in ConductorOne", Type: "boolean"},
		},
		Fetch: fetchAppUsers,
	}
}

func fetchAppUsers(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	usersRaw, err := client.ListAppUsers(appID, limit)
	if err != nil {
		return nil, err
	}
	users := parseItems(usersRaw, "appUser")

	// Fetch grants to compute grant counts (skip for small preview limits)
	grantCounts := map[string]int{}
	grantCountAvailable := false
	if limit == 0 || limit > 50 {
		grantsRaw, err := client.SearchGrants(appID, 0)
		if err == nil {
			grantCountAvailable = true
			grants := parseGrants(grantsRaw)
			for _, g := range grants {
				uid, _ := g["_appUserId"].(string)
				if uid != "" {
					grantCounts[uid]++
				}
			}
		}
		// If grants fail, grantCount will show "N/A" instead of misleading "0"
	}

	var rows []map[string]string
	for _, u := range users {
		id := strVal(u, "id")
		orphaned := "No"
		if strVal(u, "identityUserId") == "" {
			orphaned = "Yes"
		}
		gc := "N/A"
		if grantCountAvailable {
			gc = fmt.Sprintf("%d", grantCounts[id])
		}
		rows = append(rows, map[string]string{
			"id":             id,
			"displayName":    strVal(u, "displayName"),
			"email":          strVal(u, "email"),
			"username":       strVal(u, "username"),
			"identityUserId": strVal(u, "identityUserId"),
			"status":         strVal(u, "status"),
			"appUserType":    strVal(u, "appUserType"),
			"createdAt":      strVal(u, "createdAt"),
			"updatedAt":      strVal(u, "updatedAt"),
			"grantCount":     gc,
			"orphaned":       orphaned,
		})
	}
	return rows, nil
}
