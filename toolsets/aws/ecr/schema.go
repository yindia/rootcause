package awsecr

func schemaECRListRepositories() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaECRDescribeRepository() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repositoryName": map[string]any{"type": "string"},
			"region":         map[string]any{"type": "string"},
		},
		"required": []string{"repositoryName"},
	}
}

func schemaECRListImages() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repositoryName": map[string]any{"type": "string"},
			"tagStatus":      map[string]any{"type": "string", "enum": []string{"TAGGED", "UNTAGGED", "ANY"}},
			"limit":          map[string]any{"type": "number"},
			"region":         map[string]any{"type": "string"},
		},
		"required": []string{"repositoryName"},
	}
}

func schemaECRDescribeImages() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repositoryName": map[string]any{"type": "string"},
			"imageTags":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"imageDigests":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"tagStatus":      map[string]any{"type": "string", "enum": []string{"TAGGED", "UNTAGGED", "ANY"}},
			"limit":          map[string]any{"type": "number"},
			"region":         map[string]any{"type": "string"},
		},
		"required": []string{"repositoryName"},
	}
}

func schemaECRDescribeRegistry() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"region": map[string]any{"type": "string"}},
	}
}

func schemaECRGetAuthorizationToken() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"registryIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"confirm":     map[string]any{"type": "boolean"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"confirm"},
	}
}
