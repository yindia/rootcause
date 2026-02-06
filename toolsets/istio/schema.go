package istio

func schemaHealth() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func schemaConfigSummary() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
		},
	}
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

func schemaServiceMeshHosts() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
		},
	}
}

func schemaDiscoverNamespaces() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
	}
}

func schemaPodsByService() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
			"service":   map[string]any{"type": "string"},
		},
	}
}

func schemaExternalDependencyCheck() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
		},
	}
}

func schemaProxyConfig() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
			"pod":       map[string]any{"type": "string"},
			"adminPort": map[string]any{"type": "integer"},
			"format":    map[string]any{"type": "string"},
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
