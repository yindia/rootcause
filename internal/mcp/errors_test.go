package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/smithy-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBuildErrorEnvelopeTimeout(t *testing.T) {
	envelope := BuildErrorEnvelope(context.DeadlineExceeded, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "timeout" {
		t.Fatalf("expected timeout code, got %s", errMap.Code)
	}
	if !errMap.Retryable {
		t.Fatalf("expected retryable timeout")
	}
}

func TestBuildErrorEnvelopeForbidden(t *testing.T) {
	err := apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "demo", nil)
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "forbidden" {
		t.Fatalf("expected forbidden code, got %s", errMap.Code)
	}
	if errMap.Retryable {
		t.Fatalf("expected forbidden to be non-retryable")
	}
}

func TestBuildErrorEnvelopeNotFound(t *testing.T) {
	err := apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "demo")
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "not_found" {
		t.Fatalf("expected not_found code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeAWSAccessDenied(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "forbidden" {
		t.Fatalf("expected forbidden code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeCanceled(t *testing.T) {
	envelope := BuildErrorEnvelope(context.Canceled, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "canceled" {
		t.Fatalf("expected canceled code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeConflict(t *testing.T) {
	err := apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, "demo", errors.New("conflict"))
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "conflict" {
		t.Fatalf("expected conflict code, got %s", errMap.Code)
	}
	if !errMap.Retryable {
		t.Fatalf("expected conflict to be retryable")
	}
}

func TestBuildErrorEnvelopeTooManyRequests(t *testing.T) {
	err := apierrors.NewTooManyRequests("overload", 1)
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "unavailable" {
		t.Fatalf("expected unavailable code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeBadRequest(t *testing.T) {
	err := apierrors.NewBadRequest("bad request")
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "invalid_request" {
		t.Fatalf("expected invalid_request code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeAWSRateLimited(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "ThrottlingException", Message: "slow down"}
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "rate_limited" {
		t.Fatalf("expected rate_limited code, got %s", errMap.Code)
	}
	if !errMap.Retryable {
		t.Fatalf("expected rate_limited to be retryable")
	}
}

func TestBuildErrorEnvelopeAWSInvalidRequest(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "ValidationException", Message: "bad input"}
	envelope := BuildErrorEnvelope(err, map[string]any{"field": "name"})
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "invalid_request" {
		t.Fatalf("expected invalid_request code, got %s", errMap.Code)
	}
	if _, ok := envelope["details"]; !ok {
		t.Fatalf("expected details to be included")
	}
}

func TestBuildErrorEnvelopeAWSUpstreamDefault(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "Unknown", Message: "boom"}
	envelope := BuildErrorEnvelope(err, nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "upstream_error" {
		t.Fatalf("expected upstream_error code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeInvalidRequestMessage(t *testing.T) {
	envelope := BuildErrorEnvelope(errors.New("missing field"), nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "invalid_request" {
		t.Fatalf("expected invalid_request code, got %s", errMap.Code)
	}
}

func TestBuildErrorEnvelopeInternalFallback(t *testing.T) {
	envelope := BuildErrorEnvelope(errors.New("boom"), nil)
	errMap := envelope["error"].(ErrorDetail)
	if errMap.Code != "internal" {
		t.Fatalf("expected internal code, got %s", errMap.Code)
	}
}
