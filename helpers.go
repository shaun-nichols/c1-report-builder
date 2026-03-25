package main

import (
	"encoding/json"
	"fmt"
)

// unmarshalJSON is a convenience wrapper.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// strVal extracts a string value from a map, handling nested status objects.
func strVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		if s, ok := val["status"].(string); ok {
			return s
		}
		b, _ := json.Marshal(val)
		return string(b)
	case []any:
		b, _ := json.Marshal(val)
		return string(b)
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%g", val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// parseItems extracts the inner object from API view wrappers.
func parseItems(raw []json.RawMessage, innerKey string) []map[string]any {
	items := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		var outer map[string]any
		if err := json.Unmarshal(r, &outer); err != nil {
			continue
		}
		if inner, ok := outer[innerKey].(map[string]any); ok {
			items = append(items, inner)
		} else {
			items = append(items, outer)
		}
	}
	return items
}

// parseGrants handles the grant search API's nested structure.
func parseGrants(raw []json.RawMessage) []map[string]any {
	grants := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		var outer map[string]any
		if err := json.Unmarshal(r, &outer); err != nil {
			continue
		}

		g := map[string]any{}

		if entBlock, ok := outer["entitlement"].(map[string]any); ok {
			if ae, ok := entBlock["appEntitlement"].(map[string]any); ok {
				g["_entitlementId"], _ = ae["id"].(string)
				g["_entitlementDisplayName"], _ = ae["displayName"].(string)
				g["_entitlementDescription"], _ = ae["description"].(string)
				g["_entitlementAlias"], _ = ae["alias"].(string)
				g["_entitlementResourceTypeId"], _ = ae["appResourceTypeId"].(string)
			}
		}

		if binding, ok := outer["appEntitlementUserBinding"].(map[string]any); ok {
			if auBlock, ok := binding["appUser"].(map[string]any); ok {
				if au, ok := auBlock["appUser"].(map[string]any); ok {
					g["_appUserId"], _ = au["id"].(string)
					g["_appUserDisplayName"], _ = au["displayName"].(string)
					g["_appUserEmail"], _ = au["email"].(string)
					g["_appUserUsername"], _ = au["username"].(string)
					g["_appUserStatus"] = strVal(au, "status")
					g["_appUserType"], _ = au["appUserType"].(string)
					g["_identityUserId"], _ = au["identityUserId"].(string)
				}
			}
			g["createdAt"] = binding["appEntitlementUserBindingCreatedAt"]
			g["deprovisionAt"] = binding["appEntitlementUserBindingDeprovisionAt"]
			g["grantSources"] = binding["grantSources"]
		}

		grants = append(grants, g)
	}
	return grants
}
