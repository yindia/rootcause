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
	case []any:
		redacted := make([]any, 0, len(v))
		for _, item := range v {
			redacted = append(redacted, r.RedactValue(item))
		}
		return redacted
	default:
		return input
	}
}
