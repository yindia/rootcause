package redact

import (
	"regexp"
)

var (
	// Token-ish sequences (API keys, JWT fragments, etc.).
	tokenPattern = regexp.MustCompile(`(?i)([a-z0-9_\-]{20,}|eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+)`)
)

type Redactor struct{}

func New() *Redactor {
	return &Redactor{}
}

func (r *Redactor) RedactString(input string) string {
	return tokenPattern.ReplaceAllString(input, "[REDACTED]")
}

func (r *Redactor) RedactMap(input map[string]any) map[string]any {
	output := map[string]any{}
	for k, v := range input {
		output[k] = r.RedactValue(v)
	}
	return output
}

func (r *Redactor) RedactValue(input any) any {
	switch v := input.(type) {
	case string:
		return r.RedactString(v)
	case map[string]any:
		return r.RedactMap(v)
	case map[string]string:
		// Labels, headers, etc. arrive as map[string]string in tool results;
		// redact in place to preserve the original type for downstream code.
		out := make(map[string]string, len(v))
		for k, s := range v {
			out[k] = r.RedactString(s)
		}
		return out
	case []any:
		redacted := make([]any, 0, len(v))
		for _, item := range v {
			redacted = append(redacted, r.RedactValue(item))
		}
		return redacted
	case []map[string]any:
		// Observability tools (and many k8s tools) return
		// "entries"/"items"/"timeSeries" as []map[string]any. Recurse so
		// payload strings inside log entries / metric labels get redacted too.
		redacted := make([]map[string]any, 0, len(v))
		for _, item := range v {
			redacted = append(redacted, r.RedactMap(item))
		}
		return redacted
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			out = append(out, r.RedactString(s))
		}
		return out
	default:
		return input
	}
}
