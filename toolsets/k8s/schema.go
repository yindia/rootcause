package k8s

func schemaGet() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"namespace":  map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	}
}

func schemaList() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"apiVersion": map[string]any{"type": "string"},
						"kind":       map[string]any{"type": "string"},
						"resource":   map[string]any{"type": "string"},
					},
				},
			},
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
			"fieldSelector": map[string]any{"type": "string"},
		},
		"required": []string{"resources"},
	}
}

func schemaDescribe() map[string]any {
	return schemaGet()
}

func schemaDelete() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"namespace":  map[string]any{"type": "string"},
			"confirm":    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "confirm"},
	}
}

func schemaApply() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"manifest":     map[string]any{"type": "string"},
			"namespace":    map[string]any{"type": "string"},
			"fieldManager": map[string]any{"type": "string"},
			"force":        map[string]any{"type": "boolean"},
			"confirm":      map[string]any{"type": "boolean"},
		},
		"required": []string{"manifest", "confirm"},
	}
}

func schemaPatch() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"namespace":  map[string]any{"type": "string"},
			"patch":      map[string]any{"type": "string"},
			"patchType":  map[string]any{"type": "string", "enum": []string{"merge", "json", "strategic"}},
			"confirm":    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "patch", "confirm"},
	}
}

func schemaLogs() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":    map[string]any{"type": "string"},
			"pod":          map[string]any{"type": "string"},
			"container":    map[string]any{"type": "string"},
			"tailLines":    map[string]any{"type": "number"},
			"sinceSeconds": map[string]any{"type": "number"},
		},
		"required": []string{"namespace", "pod"},
	}
}

func schemaEvents() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":          map[string]any{"type": "string"},
			"involvedObjectName": map[string]any{"type": "string"},
			"involvedObjectKind": map[string]any{"type": "string"},
			"involvedObjectUID":  map[string]any{"type": "string"},
		},
	}
}

func schemaAPIResources() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"limit": map[string]any{"type": "number"},
		},
	}
}

func schemaCRDs() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"limit": map[string]any{"type": "number"},
		},
	}
}

func schemaCreate() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"manifest":  map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
			"confirm":   map[string]any{"type": "boolean"},
		},
		"required": []string{"manifest", "confirm"},
	}
}

func schemaScale() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"namespace":  map[string]any{"type": "string"},
			"replicas":   map[string]any{"type": "number"},
			"confirm":    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "replicas", "confirm"},
	}
}

func schemaRollout() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":     map[string]any{"type": "string", "enum": []string{"status", "restart"}},
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"namespace":  map[string]any{"type": "string"},
			"confirm":    map[string]any{"type": "boolean"},
		},
		"required": []string{"name", "namespace", "confirm"},
	}
}

func schemaContext() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"list", "current", "use"}},
		},
	}
}

func schemaExplain() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"apiVersion": map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"resource":   map[string]any{"type": "string"},
		},
	}
}

func schemaGraph() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":      map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
		},
		"required": []string{"kind", "name", "namespace"},
	}
}

func schemaGeneric() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"verb":          map[string]any{"type": "string"},
			"apiVersion":    map[string]any{"type": "string"},
			"kind":          map[string]any{"type": "string"},
			"resource":      map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"namespace":     map[string]any{"type": "string"},
			"manifest":      map[string]any{"type": "string"},
			"patch":         map[string]any{"type": "string"},
			"patchType":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
			"fieldSelector": map[string]any{"type": "string"},
			"pod":           map[string]any{"type": "string"},
			"container":     map[string]any{"type": "string"},
			"action":        map[string]any{"type": "string"},
			"replicas":      map[string]any{"type": "number"},
		},
		"required": []string{"verb"},
	}
}

func schemaPing() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func schemaPortForward() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":       map[string]any{"type": "string"},
			"pod":             map[string]any{"type": "string"},
			"service":         map[string]any{"type": "string"},
			"ports":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"localAddress":    map[string]any{"type": "string"},
			"durationSeconds": map[string]any{"type": "number"},
		},
		"required": []string{"namespace", "ports"},
	}
}

func schemaExec() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":  map[string]any{"type": "string"},
			"pod":        map[string]any{"type": "string"},
			"container":  map[string]any{"type": "string"},
			"command":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"allowShell": map[string]any{"type": "boolean"},
		},
		"required": []string{"namespace", "pod", "command"},
	}
}

func schemaCleanupPods() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"states":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"labelSelector": map[string]any{"type": "string"},
		},
		"required": []string{"namespace"},
	}
}

func schemaNodeManagement() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":             map[string]any{"type": "string", "enum": []string{"cordon", "uncordon", "drain"}},
			"nodeName":           map[string]any{"type": "string"},
			"gracePeriodSeconds": map[string]any{"type": "number"},
			"force":              map[string]any{"type": "boolean"},
		},
		"required": []string{"action", "nodeName"},
	}
}

func schemaDiagnose() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"keyword":   map[string]any{"type": "string"},
			"namespace": map[string]any{"type": "string"},
		},
		"required": []string{"keyword"},
	}
}

func schemaOverview() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
		},
	}
}

func schemaCrashloopDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"labelSelector": map[string]any{"type": "string"},
		},
		"required": []string{"namespace"},
	}
}

func schemaSchedulingDebug() map[string]any {
	return schemaCrashloopDebug()
}

func schemaHPADebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
		},
		"required": []string{"namespace"},
	}
}

func schemaNetworkDebug() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
			"service":   map[string]any{"type": "string"},
		},
		"required": []string{"namespace", "service"},
	}
}

func schemaPrivateLinkDebug() map[string]any {
	return schemaNetworkDebug()
}

func schemaExecReadonly() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace": map[string]any{"type": "string"},
			"pod":       map[string]any{"type": "string"},
			"container": map[string]any{"type": "string"},
			"command": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"namespace", "pod", "command"},
	}
}
