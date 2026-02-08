package audit

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLoggerWritesJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)
	logger.Log(Event{
		Timestamp: time.Unix(1, 0).UTC(),
		UserID:    "user",
		Tool:      "k8s.get",
		Toolset:   "k8s",
		Outcome:   "success",
	})
	output := buf.String()
	if !strings.Contains(output, `"tool":"k8s.get"`) {
		t.Fatalf("expected tool in output: %s", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Fatalf("expected newline")
	}
}

func TestLoggerNilWriter(t *testing.T) {
	logger := NewLogger(nil)
	logger.Log(Event{Tool: "k8s.get", Toolset: "k8s", Outcome: "success"})
}

func TestLoggerMarshalError(t *testing.T) {
	orig := jsonMarshal
	t.Cleanup(func() { jsonMarshal = orig })
	jsonMarshal = func(any) ([]byte, error) {
		return nil, fmt.Errorf("fail")
	}
	var buf bytes.Buffer
	logger := NewLogger(&buf)
	logger.Log(Event{Tool: "k8s.get", Toolset: "k8s", Outcome: "success"})
	if buf.Len() != 0 {
		t.Fatalf("expected no output on marshal error")
	}
}
