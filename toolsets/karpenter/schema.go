package karpenter

func schemaStatus() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func schemaNodeProvisioningDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
			"pods": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
}

func schemaCRStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion":    map[string]any{"type": "string"},
			"group":         map[string]any{"type": "string"},
			"kind":          map[string]any{"type": "string"},
			"resource":      map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaNodePoolDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaNodeClassDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"kind":          map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaInterruptionDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}
