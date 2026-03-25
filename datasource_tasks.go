package main

func tasksDataSource() *DataSource {
	return &DataSource{
		ID:          "tasks",
		Label:       "Tasks / Access Requests",
		Description: "Access requests, reviews, and revocations with outcomes",
		RequiresApp: false,
		Columns: []ColumnDef{
			{ID: "id", Label: "Task ID", Type: "string"},
			{ID: "numericId", Label: "Ticket #", Type: "number"},
			{ID: "displayName", Label: "Display Name", Type: "string"},
			{ID: "description", Label: "Description / Justification", Type: "string"},
			{ID: "taskType", Label: "Task Type", Type: "enum", Options: []string{"grant", "revoke", "certify", "offboarding"}},
			{ID: "state", Label: "State", Type: "enum", Options: []string{"TASK_STATE_OPEN", "TASK_STATE_CLOSED"}},
			{ID: "outcome", Label: "Outcome", Type: "string"},
			{ID: "outcomeTime", Label: "Outcome Time", Type: "date"},
			{ID: "userId", Label: "Subject User ID", Type: "string"},
			{ID: "createdByUserId", Label: "Created By User ID", Type: "string"},
			{ID: "appId", Label: "App ID", Type: "string"},
			{ID: "appEntitlementId", Label: "Entitlement ID", Type: "string"},
			{ID: "emergencyAccess", Label: "Emergency Access", Type: "boolean"},
			{ID: "createdAt", Label: "Created At", Type: "date"},
			{ID: "updatedAt", Label: "Updated At", Type: "date"},
		},
		Fetch: fetchTasks,
	}
}

func fetchTasks(client *C1Client, _ string, limit int) ([]map[string]string, error) {
	body := map[string]any{}
	raw, err := client.SearchTasks(body, limit)
	if err != nil {
		return nil, err
	}

	var rows []map[string]string
	for _, r := range raw {
		var view map[string]any
		if err := unmarshalJSON(r, &view); err != nil {
			continue
		}

		task := mapVal(view, "task")
		if task == nil {
			task = view
		}

		// Determine task type and outcome from the type sub-object
		taskType := ""
		outcome := ""
		outcomeTime := ""
		appID := ""
		entID := ""

		if g := mapVal(task, "grant"); g != nil {
			taskType = "grant"
			outcome = strVal(g, "outcome")
			outcomeTime = strVal(g, "outcomeTime")
			appID = strVal(g, "appId")
			entID = strVal(g, "appEntitlementId")
		} else if rv := mapVal(task, "revoke"); rv != nil {
			taskType = "revoke"
			outcome = strVal(rv, "outcome")
			outcomeTime = strVal(rv, "outcomeTime")
			appID = strVal(rv, "appId")
			entID = strVal(rv, "appEntitlementId")
		} else if c := mapVal(task, "certify"); c != nil {
			taskType = "certify"
			outcome = strVal(c, "outcome")
			outcomeTime = strVal(c, "outcomeTime")
			appID = strVal(c, "appId")
			entID = strVal(c, "appEntitlementId")
		} else if o := mapVal(task, "offboarding"); o != nil {
			taskType = "offboarding"
			outcome = strVal(o, "outcome")
			outcomeTime = strVal(o, "outcomeTime")
		}

		// Check nested type object if flat fields not present
		if taskType == "" {
			if typeObj := mapVal(task, "type"); typeObj != nil {
				if g := mapVal(typeObj, "grant"); g != nil {
					taskType = "grant"
					outcome = strVal(g, "outcome")
					outcomeTime = strVal(g, "outcomeTime")
					appID = strVal(g, "appId")
					entID = strVal(g, "appEntitlementId")
				} else if rv := mapVal(typeObj, "revoke"); rv != nil {
					taskType = "revoke"
					outcome = strVal(rv, "outcome")
					outcomeTime = strVal(rv, "outcomeTime")
					appID = strVal(rv, "appId")
					entID = strVal(rv, "appEntitlementId")
				} else if c := mapVal(typeObj, "certify"); c != nil {
					taskType = "certify"
					outcome = strVal(c, "outcome")
					outcomeTime = strVal(c, "outcomeTime")
					appID = strVal(c, "appId")
					entID = strVal(c, "appEntitlementId")
				} else if o := mapVal(typeObj, "offboarding"); o != nil {
					taskType = "offboarding"
					outcome = strVal(o, "outcome")
					outcomeTime = strVal(o, "outcomeTime")
				}
			}
		}

		emergency := "No"
		if strVal(task, "emergencyAccess") == "true" {
			emergency = "Yes"
		}

		rows = append(rows, map[string]string{
			"id":               strVal(task, "id"),
			"numericId":        strVal(task, "numericId"),
			"displayName":      strVal(task, "displayName"),
			"description":      strVal(task, "description"),
			"taskType":         taskType,
			"state":            strVal(task, "state"),
			"outcome":          outcome,
			"outcomeTime":      outcomeTime,
			"userId":           strVal(task, "userId"),
			"createdByUserId":  strVal(task, "createdByUserId"),
			"appId":            appID,
			"appEntitlementId": entID,
			"emergencyAccess":  emergency,
			"createdAt":        strVal(task, "createdAt"),
			"updatedAt":        strVal(task, "updatedAt"),
		})
	}
	return rows, nil
}

func mapVal(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}
