package awssts

func schemaSTSGetCallerIdentity() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"region": map[string]any{"type": "string"}},
	}
}

func schemaSTSAssumeRole() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"roleArn":         map[string]any{"type": "string"},
			"sessionName":     map[string]any{"type": "string"},
			"durationSeconds": map[string]any{"type": "number"},
			"externalId":      map[string]any{"type": "string"},
			"policy":          map[string]any{"type": "string"},
			"confirm":         map[string]any{"type": "boolean"},
			"region":          map[string]any{"type": "string"},
		},
		"required": []string{"roleArn", "sessionName", "confirm"},
	}
}
