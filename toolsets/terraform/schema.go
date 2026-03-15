package terraform

func schemaListModules() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"provider":    map[string]any{"type": "string"},
			"verified":    map[string]any{"type": "boolean"},
			"limit":       map[string]any{"type": "number"},
			"offset":      map[string]any{"type": "number"},
		},
	}
}

func schemaGetModule() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"provider":    map[string]any{"type": "string"},
			"version":     map[string]any{"type": "string"},
		},
		"required": []string{"namespace", "name", "provider"},
	}
}

func schemaListModuleVersions() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"provider":    map[string]any{"type": "string"},
		},
		"required": []string{"namespace", "name", "provider"},
	}
}

func schemaSearchModules() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"query":       map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"provider":    map[string]any{"type": "string"},
			"verified":    map[string]any{"type": "boolean"},
			"limit":       map[string]any{"type": "number"},
			"offset":      map[string]any{"type": "number"},
		},
		"required": []string{"query"},
	}
}

func schemaListProviders() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"tier":        map[string]any{"type": "string"},
			"pageSize":    map[string]any{"type": "number"},
			"pageNumber":  map[string]any{"type": "number"},
			"limit":       map[string]any{"type": "number"},
		},
	}
}

func schemaGetProvider() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":     map[string]any{"type": "string"},
			"namespace":       map[string]any{"type": "string"},
			"type":            map[string]any{"type": "string"},
			"version":         map[string]any{"type": "string"},
			"allowPrerelease": map[string]any{"type": "boolean"},
		},
		"required": []string{"namespace", "type"},
	}
}

func schemaListProviderVersions() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":     map[string]any{"type": "string"},
			"namespace":       map[string]any{"type": "string"},
			"type":            map[string]any{"type": "string"},
			"allowPrerelease": map[string]any{"type": "boolean"},
			"limit":           map[string]any{"type": "number"},
		},
		"required": []string{"namespace", "type"},
	}
}

func schemaGetProviderPackage() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"version":     map[string]any{"type": "string"},
			"os":          map[string]any{"type": "string"},
			"arch":        map[string]any{"type": "string"},
		},
		"required": []string{"namespace", "type", "version", "os", "arch"},
	}
}

func schemaSearchProviders() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL": map[string]any{"type": "string"},
			"query":       map[string]any{"type": "string"},
			"namespace":   map[string]any{"type": "string"},
			"tier":        map[string]any{"type": "string"},
			"pageSize":    map[string]any{"type": "number"},
			"pageNumber":  map[string]any{"type": "number"},
			"limit":       map[string]any{"type": "number"},
		},
		"required": []string{"query"},
	}
}

func schemaListResources() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"includeContent":    map[string]any{"type": "boolean"},
			"pageSize":          map[string]any{"type": "number"},
			"pageNumber":        map[string]any{"type": "number"},
			"limit":             map[string]any{"type": "number"},
		},
		"required": []string{"providerNamespace", "providerType"},
	}
}

func schemaGetResource() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"resourceType":      map[string]any{"type": "string"},
		},
		"required": []string{"providerNamespace", "providerType", "resourceType"},
	}
}

func schemaSearchResources() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"query":             map[string]any{"type": "string"},
			"includeContent":    map[string]any{"type": "boolean"},
			"pageSize":          map[string]any{"type": "number"},
			"pageNumber":        map[string]any{"type": "number"},
			"limit":             map[string]any{"type": "number"},
		},
		"required": []string{"providerNamespace", "providerType", "query"},
	}
}

func schemaListDataSources() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"includeContent":    map[string]any{"type": "boolean"},
			"pageSize":          map[string]any{"type": "number"},
			"pageNumber":        map[string]any{"type": "number"},
			"limit":             map[string]any{"type": "number"},
		},
		"required": []string{"providerNamespace", "providerType"},
	}
}

func schemaGetDataSource() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"dataSourceType":    map[string]any{"type": "string"},
		},
		"required": []string{"providerNamespace", "providerType", "dataSourceType"},
	}
}

func schemaSearchDataSources() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryURL":       map[string]any{"type": "string"},
			"providerNamespace": map[string]any{"type": "string"},
			"providerType":      map[string]any{"type": "string"},
			"providerVersion":   map[string]any{"type": "string"},
			"allowPrerelease":   map[string]any{"type": "boolean"},
			"query":             map[string]any{"type": "string"},
			"includeContent":    map[string]any{"type": "boolean"},
			"pageSize":          map[string]any{"type": "number"},
			"pageNumber":        map[string]any{"type": "number"},
			"limit":             map[string]any{"type": "number"},
		},
		"required": []string{"providerNamespace", "providerType", "query"},
	}
}

func schemaDebugPlan() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"plan":                map[string]any{"type": "object"},
			"planJSON":            map[string]any{"type": "string"},
			"focusAddresses":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"includeNoOp":         map[string]any{"type": "boolean"},
			"summarizeByProvider": map[string]any{"type": "boolean"},
		},
	}
}
