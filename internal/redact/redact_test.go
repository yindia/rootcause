package redact

import "testing"

func TestRedactString(t *testing.T) {
	r := New()
	input := "token=abcdEFGHijklMNOPqrst"
	out := r.RedactString(input)
	if out == input {
		t.Fatalf("expected redaction")
	}
	if out != "token=[REDACTED]" {
		t.Fatalf("unexpected redaction output: %s", out)
	}
}

func TestRedactValueNested(t *testing.T) {
	r := New()
	in := map[string]any{
		"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xxx.yyy",
		"list":  []any{"keep", "abcdefghijklmnopqrstuvwxyz123456"},
	}
	out := r.RedactValue(in).(map[string]any)
	if out["token"] == in["token"] {
		t.Fatalf("expected token redacted")
	}
	list := out["list"].([]any)
	if list[1] == "abcdefghijklmnopqrstuvwxyz123456" {
		t.Fatalf("expected list entry redacted")
	}
}
