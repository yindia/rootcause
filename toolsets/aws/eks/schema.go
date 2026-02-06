package awseks

func schemaEKSListClusters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "number"},
			"region": map[string]any{"type": "string"},
		},
	}
}

func schemaEKSGetCluster() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":   map[string]any{"type": "string"},
			"region": map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	}
}

func schemaEKSListNodegroups() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"limit":       map[string]any{"type": "number"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}

func schemaEKSGetNodegroup() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName":   map[string]any{"type": "string"},
			"nodegroupName": map[string]any{"type": "string"},
			"region":        map[string]any{"type": "string"},
		},
		"required": []string{"clusterName", "nodegroupName"},
	}
}

func schemaEKSListAddons() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"limit":       map[string]any{"type": "number"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}

func schemaEKSGetAddon() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"addonName":   map[string]any{"type": "string"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName", "addonName"},
	}
}

func schemaEKSListFargateProfiles() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"limit":       map[string]any{"type": "number"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}

func schemaEKSGetFargateProfile() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"profileName": map[string]any{"type": "string"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName", "profileName"},
	}
}

func schemaEKSListIdentityProviderConfigs() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"limit":       map[string]any{"type": "number"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}

func schemaEKSGetIdentityProviderConfig() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName": map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"region":      map[string]any{"type": "string"},
		},
		"required": []string{"clusterName", "type", "name"},
	}
}

func schemaEKSListUpdates() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName":   map[string]any{"type": "string"},
			"nodegroupName": map[string]any{"type": "string"},
			"limit":         map[string]any{"type": "number"},
			"region":        map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}

func schemaEKSGetUpdate() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName":   map[string]any{"type": "string"},
			"nodegroupName": map[string]any{"type": "string"},
			"updateId":      map[string]any{"type": "string"},
			"region":        map[string]any{"type": "string"},
		},
		"required": []string{"clusterName", "updateId"},
	}
}

func schemaEKSListNodes() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"clusterName":   map[string]any{"type": "string"},
			"nodegroupName": map[string]any{"type": "string"},
			"limit":         map[string]any{"type": "number"},
			"region":        map[string]any{"type": "string"},
		},
		"required": []string{"clusterName"},
	}
}
