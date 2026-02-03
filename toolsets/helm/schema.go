package helm

func schemaRepoAdd() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":                  map[string]any{"type": "string"},
			"url":                   map[string]any{"type": "string"},
			"username":              map[string]any{"type": "string"},
			"password":              map[string]any{"type": "string"},
			"caFile":                map[string]any{"type": "string"},
			"certFile":              map[string]any{"type": "string"},
			"keyFile":               map[string]any{"type": "string"},
			"insecureSkipTLSVerify": map[string]any{"type": "boolean"},
			"passCredentialsAll":    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "url"},
	}
}

func schemaRepoList() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func schemaRepoUpdate() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
}

func schemaList() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"allNamespaces": map[string]any{"type": "boolean"},
			"filter":        map[string]any{"type": "string"},
			"limit":         map[string]any{"type": "number"},
		},
	}
}

func schemaStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":   map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
		},
		"required": []string{"release", "namespace"},
	}
}

func schemaInstall() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":               map[string]any{"type": "string"},
			"chart":                 map[string]any{"type": "string"},
			"namespace":             map[string]any{"type": "string"},
			"version":               map[string]any{"type": "string"},
			"repoURL":               map[string]any{"type": "string"},
			"username":              map[string]any{"type": "string"},
			"password":              map[string]any{"type": "string"},
			"caFile":                map[string]any{"type": "string"},
			"certFile":              map[string]any{"type": "string"},
			"keyFile":               map[string]any{"type": "string"},
			"insecureSkipTLSVerify": map[string]any{"type": "boolean"},
			"passCredentialsAll":    map[string]any{"type": "boolean"},
			"values":                map[string]any{"type": "object"},
			"valuesYAML":            map[string]any{"type": "string"},
			"valuesFiles":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"createNamespace":       map[string]any{"type": "boolean"},
			"wait":                  map[string]any{"type": "boolean"},
			"atomic":                map[string]any{"type": "boolean"},
			"timeoutSeconds":        map[string]any{"type": "number"},
			"includeCRDs":           map[string]any{"type": "boolean"},
			"confirm":               map[string]any{"type": "boolean"},
		},
		"required": []string{"release", "chart", "namespace", "confirm"},
	}
}

func schemaUpgrade() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":               map[string]any{"type": "string"},
			"chart":                 map[string]any{"type": "string"},
			"namespace":             map[string]any{"type": "string"},
			"version":               map[string]any{"type": "string"},
			"repoURL":               map[string]any{"type": "string"},
			"username":              map[string]any{"type": "string"},
			"password":              map[string]any{"type": "string"},
			"caFile":                map[string]any{"type": "string"},
			"certFile":              map[string]any{"type": "string"},
			"keyFile":               map[string]any{"type": "string"},
			"insecureSkipTLSVerify": map[string]any{"type": "boolean"},
			"passCredentialsAll":    map[string]any{"type": "boolean"},
			"values":                map[string]any{"type": "object"},
			"valuesYAML":            map[string]any{"type": "string"},
			"valuesFiles":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"install":               map[string]any{"type": "boolean"},
			"wait":                  map[string]any{"type": "boolean"},
			"atomic":                map[string]any{"type": "boolean"},
			"timeoutSeconds":        map[string]any{"type": "number"},
			"includeCRDs":           map[string]any{"type": "boolean"},
			"confirm":               map[string]any{"type": "boolean"},
		},
		"required": []string{"release", "chart", "namespace", "confirm"},
	}
}

func schemaUninstall() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":        map[string]any{"type": "string"},
			"namespace":      map[string]any{"type": "string"},
			"keepHistory":    map[string]any{"type": "boolean"},
			"timeoutSeconds": map[string]any{"type": "number"},
			"confirm":        map[string]any{"type": "boolean"},
		},
		"required": []string{"release", "namespace", "confirm"},
	}
}

func schemaTemplateApply() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":               map[string]any{"type": "string"},
			"chart":                 map[string]any{"type": "string"},
			"namespace":             map[string]any{"type": "string"},
			"version":               map[string]any{"type": "string"},
			"repoURL":               map[string]any{"type": "string"},
			"username":              map[string]any{"type": "string"},
			"password":              map[string]any{"type": "string"},
			"caFile":                map[string]any{"type": "string"},
			"certFile":              map[string]any{"type": "string"},
			"keyFile":               map[string]any{"type": "string"},
			"insecureSkipTLSVerify": map[string]any{"type": "boolean"},
			"passCredentialsAll":    map[string]any{"type": "boolean"},
			"values":                map[string]any{"type": "object"},
			"valuesYAML":            map[string]any{"type": "string"},
			"valuesFiles":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"includeCRDs":           map[string]any{"type": "boolean"},
			"fieldManager":          map[string]any{"type": "string"},
			"force":                 map[string]any{"type": "boolean"},
			"confirm":               map[string]any{"type": "boolean"},
		},
		"required": []string{"release", "chart", "namespace", "confirm"},
	}
}

func schemaTemplateUninstall() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"release":               map[string]any{"type": "string"},
			"chart":                 map[string]any{"type": "string"},
			"namespace":             map[string]any{"type": "string"},
			"version":               map[string]any{"type": "string"},
			"repoURL":               map[string]any{"type": "string"},
			"username":              map[string]any{"type": "string"},
			"password":              map[string]any{"type": "string"},
			"caFile":                map[string]any{"type": "string"},
			"certFile":              map[string]any{"type": "string"},
			"keyFile":               map[string]any{"type": "string"},
			"insecureSkipTLSVerify": map[string]any{"type": "boolean"},
			"passCredentialsAll":    map[string]any{"type": "boolean"},
			"values":                map[string]any{"type": "object"},
			"valuesYAML":            map[string]any{"type": "string"},
			"valuesFiles":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"includeCRDs":           map[string]any{"type": "boolean"},
			"confirm":               map[string]any{"type": "boolean"},
		},
		"required": []string{"release", "chart", "namespace", "confirm"},
	}
}
