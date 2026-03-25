package main

import "encoding/json"

func systemLogDataSource() *DataSource {
	return &DataSource{
		ID:          "system_log",
		Label:       "System Log (Audit Trail)",
		Description: "OCSF-formatted audit events — admin actions, access changes, authentication",
		RequiresApp: false,
		Columns: []ColumnDef{
			{ID: "time", Label: "Time", Type: "date"},
			{ID: "type_uid", Label: "Event Type UID", Type: "string"},
			{ID: "type_name", Label: "Event Type", Type: "string"},
			{ID: "activity_name", Label: "Activity", Type: "string"},
			{ID: "severity", Label: "Severity", Type: "string"},
			{ID: "status", Label: "Status", Type: "string"},
			{ID: "message", Label: "Message", Type: "string"},
			{ID: "actor_user", Label: "Actor", Type: "string"},
			{ID: "target", Label: "Target", Type: "string"},
			{ID: "raw", Label: "Raw Event (JSON)", Type: "string"},
		},
		Fetch: fetchSystemLog,
	}
}

func fetchSystemLog(client *C1Client, _ string, limit int) ([]map[string]string, error) {
	raw, err := client.ListSystemLogEvents(limit)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, r := range raw {
		var event map[string]any
		if err := json.Unmarshal(r, &event); err != nil {
			continue
		}

		// OCSF events have flexible schema — extract common fields
		actor := ""
		if actorObj := mapVal(event, "actor"); actorObj != nil {
			if user := mapVal(actorObj, "user"); user != nil {
				actor = strVal(user, "email_addr")
				if actor == "" {
					actor = strVal(user, "name")
				}
			}
		}

		target := ""
		if targetObj := mapVal(event, "target"); targetObj != nil {
			target = strVal(targetObj, "name")
			if target == "" {
				target = strVal(targetObj, "uid")
			}
		}

		rawJSON, _ := json.Marshal(event)

		rows = append(rows, map[string]string{
			"time":          strVal(event, "time"),
			"type_uid":      strVal(event, "type_uid"),
			"type_name":     strVal(event, "type_name"),
			"activity_name": strVal(event, "activity_name"),
			"severity":      strVal(event, "severity"),
			"status":        strVal(event, "status"),
			"message":       strVal(event, "message"),
			"actor_user":    actor,
			"target":        target,
			"raw":           string(rawJSON),
		})
	}
	return rows, nil
}
