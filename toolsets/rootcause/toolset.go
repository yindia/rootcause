package rootcause

import (
	"errors"
	"fmt"
	"time"

	"rootcause/internal/mcp"
)

type Toolset struct {
	ctx mcp.ToolsetContext
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("rootcause", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "rootcause"
}

func (t *Toolset) Version() string {
	return "0.1.0"
}

func (t *Toolset) Init(ctx mcp.ToolsetContext) error {
	if ctx.Invoker == nil {
		return errors.New("missing tool invoker")
	}
	t.ctx = ctx
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	spec := mcp.ToolSpec{
		Name:        "rootcause.incident_bundle",
		Description: "Aggregate incident context across k8s, helm, and diagnose flows.",
		ToolsetID:   t.ID(),
		InputSchema: schemaIncidentBundle(),
		Safety:      mcp.SafetyReadOnly,
		Handler:     t.handleIncidentBundle,
	}
	if err := reg.Add(spec); err != nil {
		return fmt.Errorf("register %s: %w", spec.Name, err)
	}
	rcaSpec := mcp.ToolSpec{
		Name:        "rootcause.rca_generate",
		Description: "Generate an RCA draft from incident evidence and bundle output.",
		ToolsetID:   t.ID(),
		InputSchema: schemaRCAGenerate(),
		Safety:      mcp.SafetyReadOnly,
		Handler:     t.handleRCAGenerate,
	}
	if err := reg.Add(rcaSpec); err != nil {
		return fmt.Errorf("register %s: %w", rcaSpec.Name, err)
	}
	playbookSpec := mcp.ToolSpec{
		Name:        "rootcause.remediation_playbook",
		Description: "Generate a prioritized remediation playbook from incident and RCA evidence.",
		ToolsetID:   t.ID(),
		InputSchema: schemaRemediationPlaybook(),
		Safety:      mcp.SafetyReadOnly,
		Handler:     t.handleRemediationPlaybook,
	}
	if err := reg.Add(playbookSpec); err != nil {
		return fmt.Errorf("register %s: %w", playbookSpec.Name, err)
	}
	postmortemSpec := mcp.ToolSpec{
		Name:        "rootcause.postmortem_export",
		Description: "Export a structured postmortem document from bundle and RCA data.",
		ToolsetID:   t.ID(),
		InputSchema: schemaPostmortemExport(),
		Safety:      mcp.SafetyReadOnly,
		Handler:     t.handlePostmortemExport,
	}
	if err := reg.Add(postmortemSpec); err != nil {
		return fmt.Errorf("register %s: %w", postmortemSpec.Name, err)
	}
	changeTimelineSpec := mcp.ToolSpec{
		Name:        "rootcause.change_timeline",
		Description: "Build a unified incident change timeline from k8s events and Helm release updates.",
		ToolsetID:   t.ID(),
		InputSchema: schemaChangeTimeline(),
		Safety:      mcp.SafetyReadOnly,
		Handler:     t.handleChangeTimeline,
	}
	if err := reg.Add(changeTimelineSpec); err != nil {
		return fmt.Errorf("register %s: %w", changeTimelineSpec.Name, err)
	}
	return nil
}

func schemaIncidentBundle() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":           map[string]any{"type": "string"},
			"keyword":             map[string]any{"type": "string"},
			"outputMode":          map[string]any{"type": "string", "enum": []string{"bundle", "timeline"}},
			"eventLimit":          map[string]any{"type": "number"},
			"releaseLimit":        map[string]any{"type": "number"},
			"includeHelm":         map[string]any{"type": "boolean"},
			"includeDefaultChain": map[string]any{"type": "boolean"},
			"continueOnError":     map[string]any{"type": "boolean"},
			"maxSteps":            map[string]any{"type": "number"},
			"toolChain": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool":    map[string]any{"type": "string"},
						"section": map[string]any{"type": "string"},
						"args":    map[string]any{"type": "object"},
					},
					"required": []string{"tool"},
				},
			},
		},
	}
}

func schemaRCAGenerate() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bundle":          map[string]any{"type": "object"},
			"namespace":       map[string]any{"type": "string"},
			"keyword":         map[string]any{"type": "string"},
			"incidentSummary": map[string]any{"type": "string"},
		},
	}
}

func schemaRemediationPlaybook() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bundle":              map[string]any{"type": "object"},
			"rca":                 map[string]any{"type": "object"},
			"namespace":           map[string]any{"type": "string"},
			"keyword":             map[string]any{"type": "string"},
			"maxImmediateActions": map[string]any{"type": "number"},
		},
	}
}

func schemaPostmortemExport() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"bundle":          map[string]any{"type": "object"},
			"rca":             map[string]any{"type": "object"},
			"namespace":       map[string]any{"type": "string"},
			"keyword":         map[string]any{"type": "string"},
			"incidentSummary": map[string]any{"type": "string"},
			"format":          map[string]any{"type": "string", "enum": []string{"json", "markdown"}},
		},
	}
}

func schemaChangeTimeline() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"namespace":     map[string]any{"type": "string"},
			"keyword":       map[string]any{"type": "string"},
			"eventLimit":    map[string]any{"type": "number"},
			"releaseLimit":  map[string]any{"type": "number"},
			"timelineLimit": map[string]any{"type": "number"},
			"includeHelm":   map[string]any{"type": "boolean"},
			"includeNormal": map[string]any{"type": "boolean"},
		},
	}
}

func boolOrDefault(value any, fallback bool) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return fallback
}

func intOrDefault(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprintf("%v", value)
}
