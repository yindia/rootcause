package awsiam

func schemaIAMListRoles() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pathPrefix": map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "number"},
			"region":     map[string]any{"type": "string"},
		},
	}
}

func schemaIAMGetRole() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roleName":        map[string]any{"type": "string"},
			"includePolicies": map[string]any{"type": "boolean"},
			"region":          map[string]any{"type": "string"},
		},
		"required": []string{"roleName"},
	}
}

func schemaIAMUpdateRole() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roleName":                  map[string]any{"type": "string"},
			"description":               map[string]any{"type": "string"},
			"assumeRolePolicyDocument":  map[string]any{"type": "string"},
			"maxSessionDurationSeconds": map[string]any{"type": "number"},
			"confirm":                   map[string]any{"type": "boolean"},
			"region":                    map[string]any{"type": "string"},
		},
		"required": []string{"roleName", "confirm"},
	}
}

func schemaIAMDeleteRole() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roleName": map[string]any{"type": "string"},
			"force":    map[string]any{"type": "boolean"},
			"confirm":  map[string]any{"type": "boolean"},
			"region":   map[string]any{"type": "string"},
		},
		"required": []string{"roleName", "confirm"},
	}
}

func schemaIAMListPolicies() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scope":        map[string]any{"type": "string", "enum": []string{"All", "AWS", "Local"}},
			"onlyAttached": map[string]any{"type": "boolean"},
			"pathPrefix":   map[string]any{"type": "string"},
			"limit":        map[string]any{"type": "number"},
			"region":       map[string]any{"type": "string"},
		},
	}
}

func schemaIAMGetPolicy() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"policyArn":       map[string]any{"type": "string"},
			"versionId":       map[string]any{"type": "string"},
			"includeDocument": map[string]any{"type": "boolean"},
			"region":          map[string]any{"type": "string"},
		},
		"required": []string{"policyArn"},
	}
}

func schemaIAMUpdatePolicy() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"policyArn":  map[string]any{"type": "string"},
			"document":   map[string]any{"type": "string"},
			"setDefault": map[string]any{"type": "boolean"},
			"prune":      map[string]any{"type": "boolean"},
			"confirm":    map[string]any{"type": "boolean"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"policyArn", "document", "confirm"},
	}
}

func schemaIAMDeletePolicy() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"policyArn": map[string]any{"type": "string"},
			"force":     map[string]any{"type": "boolean"},
			"confirm":   map[string]any{"type": "boolean"},
			"region":    map[string]any{"type": "string"},
		},
		"required": []string{"policyArn", "confirm"},
	}
}
