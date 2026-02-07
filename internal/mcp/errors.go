package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/smithy-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable"`
}

type ErrorEnvelope struct {
	Error   ErrorDetail `json:"error"`
	Details any         `json:"details,omitempty"`
}

func BuildErrorEnvelope(err error, details any) map[string]any {
	envelope := ErrorEnvelope{Error: classifyError(err)}
	out := map[string]any{"error": envelope.Error}
	if details != nil {
		out["details"] = details
	}
	return out
}

func classifyError(err error) ErrorDetail {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorDetail{Code: "timeout", Message: msg, Hint: "Increase the timeout or check cluster/network latency.", Retryable: true}
	}
	if errors.Is(err, context.Canceled) {
		return ErrorDetail{Code: "canceled", Message: msg, Hint: "Request was canceled before completion.", Retryable: true}
	}
	if apierrors.IsUnauthorized(err) {
		return ErrorDetail{Code: "unauthorized", Message: msg, Hint: "Check credentials or auth configuration.", Retryable: false}
	}
	if apierrors.IsForbidden(err) {
		return ErrorDetail{Code: "forbidden", Message: msg, Hint: "Check permissions or namespace access.", Retryable: false}
	}
	if apierrors.IsNotFound(err) {
		return ErrorDetail{Code: "not_found", Message: msg, Hint: "Verify the resource name/namespace.", Retryable: false}
	}
	if apierrors.IsConflict(err) {
		return ErrorDetail{Code: "conflict", Message: msg, Hint: "Resource update conflict; retry with latest state.", Retryable: true}
	}
	if apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) {
		return ErrorDetail{Code: "unavailable", Message: msg, Hint: "API server overloaded; retry with backoff.", Retryable: true}
	}
	if apierrors.IsBadRequest(err) {
		return ErrorDetail{Code: "invalid_request", Message: msg, Hint: "Fix request parameters or schema.", Retryable: false}
	}

	if apiErr, ok := err.(smithy.APIError); ok {
		code := apiErr.ErrorCode()
		switch code {
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation":
			return ErrorDetail{Code: "forbidden", Message: msg, Hint: "Check AWS credentials and IAM policies.", Retryable: false}
		case "Throttling", "ThrottlingException", "RequestLimitExceeded", "TooManyRequestsException":
			return ErrorDetail{Code: "rate_limited", Message: msg, Hint: "Retry with backoff.", Retryable: true}
		case "ResourceNotFoundException", "NotFoundException", "NoSuchEntity":
			return ErrorDetail{Code: "not_found", Message: msg, Hint: "Verify resource identifiers and region.", Retryable: false}
		case "ValidationException", "InvalidParameterException", "InvalidParameterValue":
			return ErrorDetail{Code: "invalid_request", Message: msg, Hint: "Fix request parameters or schema.", Retryable: false}
		case "ConflictException":
			return ErrorDetail{Code: "conflict", Message: msg, Hint: "Resource update conflict; retry.", Retryable: true}
		default:
			return ErrorDetail{Code: "upstream_error", Message: msg, Hint: "AWS API error; verify inputs and retry.", Retryable: true}
		}
	}

	if isInvalidRequestMessage(msg) {
		return ErrorDetail{Code: "invalid_request", Message: msg, Hint: "Fix request parameters or schema.", Retryable: false}
	}

	return ErrorDetail{Code: "internal", Message: msg, Hint: "Check server logs for details.", Retryable: false}
}

func isInvalidRequestMessage(msg string) bool {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "required") || strings.Contains(lower, "invalid") || strings.Contains(lower, "missing") {
		return true
	}
	return false
}
