package istio

func schemaHealth() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func schemaProxyStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
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

func schemaHTTPRouteStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion":    map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaGatewayStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion":    map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaVirtualServiceStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion":    map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaDestinationRuleStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion":    map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}
