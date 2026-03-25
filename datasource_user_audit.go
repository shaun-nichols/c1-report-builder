package main

import (
	"fmt"
	"sort"
	"strings"
)

func userAuditDataSource() *DataSource {
	return &DataSource{
		ID:          "user_audit",
		Label:       "User Audit (Combined)",
		Description: "One row per user with all entitlements and grant details consolidated — matches the original c1-report-tool output",
		RequiresApp: true,
		Columns: []ColumnDef{
			{ID: "appUserId", Label: "App User ID", Type: "string"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "email", Label: "Email", Type: "string"},
			{ID: "username", Label: "Username", Type: "string"},
			{ID: "identityUserId", Label: "Identity User ID", Type: "string"},
			{ID: "orphaned", Label: "Orphaned", Description: "No identity mapping in ConductorOne", Type: "boolean"},
			{ID: "status", Label: "Status", Type: "enum", Options: []string{"STATUS_ENABLED", "STATUS_DISABLED"}},
			{ID: "appUserType", Label: "User Type", Type: "enum", Options: []string{"APP_USER_TYPE_USER", "APP_USER_TYPE_SERVICE_ACCOUNT", "APP_USER_TYPE_SYSTEM_ACCOUNT"}},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
			{ID: "grantCount", Label: "Grant Count", Type: "number"},
			{ID: "entitlements", Label: "Entitlements", Description: "Semicolon-separated list of entitlement names", Type: "string"},
			{ID: "grantDetails", Label: "Grant Details", Description: "Entitlement name, granted date, and expiration per grant", Type: "string"},
		},
		Fetch: fetchUserAudit,
	}
}

func fetchUserAudit(client *C1Client, appID string, limit int) ([]map[string]string, error) {
	// Fetch users
	usersRaw, err := client.ListAppUsers(appID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching app users: %w", err)
	}
	users := parseItems(usersRaw, "appUser")

	// Fetch entitlements for name lookup
	entsRaw, err := client.ListEntitlements(appID, 0)
	if err != nil {
		return nil, fmt.Errorf("fetching entitlements: %w", err)
	}
	ents := parseItems(entsRaw, "appEntitlement")
	entsByID := map[string]map[string]any{}
	for _, e := range ents {
		if id := strVal(e, "id"); id != "" {
			entsByID[id] = e
		}
	}

	// Fetch grants
	grantsRaw, err := client.SearchGrants(appID, 0)
	if err != nil {
		return nil, fmt.Errorf("fetching grants: %w", err)
	}
	grants := parseGrants(grantsRaw)

	// Group grants by user
	grantsByUser := map[string][]map[string]any{}
	for _, g := range grants {
		uid, _ := g["_appUserId"].(string)
		if uid != "" {
			grantsByUser[uid] = append(grantsByUser[uid], g)
		}
	}

	// Sort users by display name
	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(strVal(users[i], "displayName")) < strings.ToLower(strVal(users[j], "displayName"))
	})

	var rows []map[string]string
	for _, u := range users {
		uid := strVal(u, "id")
		userGrants := grantsByUser[uid]

		// Sort grants by entitlement name
		sort.Slice(userGrants, func(i, j int) bool {
			eiID, _ := userGrants[i]["_entitlementId"].(string)
			ejID, _ := userGrants[j]["_entitlementId"].(string)
			ei := entsByID[eiID]
			ej := entsByID[ejID]
			return strings.ToLower(strVal(ei, "displayName")) < strings.ToLower(strVal(ej, "displayName"))
		})

		var entNames []string
		var grantDetails []string
		for _, g := range userGrants {
			eid, _ := g["_entitlementId"].(string)
			ent := entsByID[eid]
			entName := strVal(ent, "displayName")
			entNames = append(entNames, entName)

			detail := entName
			granted := strVal(g, "createdAt")
			expires := strVal(g, "deprovisionAt")
			if granted != "" {
				detail += " | granted: " + granted
			}
			if expires != "" {
				detail += " | expires: " + expires
			}
			grantDetails = append(grantDetails, detail)
		}

		orphaned := "No"
		if strVal(u, "identityUserId") == "" {
			orphaned = "Yes"
		}

		rows = append(rows, map[string]string{
			"appUserId":      uid,
			"displayName":    strVal(u, "displayName"),
			"email":          strVal(u, "email"),
			"username":       strVal(u, "username"),
			"identityUserId": strVal(u, "identityUserId"),
			"orphaned":       orphaned,
			"status":         strVal(u, "status"),
			"appUserType":    strVal(u, "appUserType"),
			"createdAt":      strVal(u, "createdAt"),
			"updatedAt":      strVal(u, "updatedAt"),
			"grantCount":     fmt.Sprintf("%d", len(userGrants)),
			"entitlements":   strings.Join(entNames, "; "),
			"grantDetails":   strings.Join(grantDetails, "\n"),
		})
	}
	return rows, nil
}
