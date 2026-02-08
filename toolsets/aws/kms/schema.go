package awskms

func schemaKMSListKeys() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaKMSListAliases() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaKMSDescribeKey() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyId":  map[string]any{"type": "string"},
			"region": map[string]any{"type": "string"},
		},
		"required": []string{"keyId"},
	}
}

func schemaKMSGetKeyPolicy() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyId":      map[string]any{"type": "string"},
			"policyName": map[string]any{"type": "string"},
			"region":     map[string]any{"type": "string"},
		},
		"required": []string{"keyId"},
	}
}
