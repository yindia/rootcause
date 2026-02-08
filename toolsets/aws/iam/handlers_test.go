package awsiam

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestIAMHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	called := false
	svc := &Service{
		ctx: ctx,
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			called = true
			return nil, "", nil
		},
	}

	tests := []struct {
		name    string
		handler func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{
			name:    "getRoleMissingName",
			handler: svc.handleIAMGetRole,
			args:    map[string]any{},
			wantErr: "roleName is required",
		},
		{
			name:    "updateRoleConfirm",
			handler: svc.handleIAMUpdateRole,
			args:    map[string]any{"roleName": "demo"},
			wantErr: "confirmation required",
		},
		{
			name:    "updateRoleInvalidJSON",
			handler: svc.handleIAMUpdateRole,
			args: map[string]any{
				"roleName":                 "demo",
				"confirm":                  true,
				"assumeRolePolicyDocument": "{not-json}",
			},
			wantErr: "assumeRolePolicyDocument must be valid JSON",
		},
		{
			name:    "updatePolicyInvalidJSON",
			handler: svc.handleIAMUpdatePolicy,
			args: map[string]any{
				"policyArn": "arn:aws:iam::123:policy/demo",
				"document":  "{oops}",
				"confirm":   true,
			},
			wantErr: "document must be valid JSON",
		},
		{
			name:    "deletePolicyConfirm",
			handler: svc.handleIAMDeletePolicy,
			args:    map[string]any{"policyArn": "arn:aws:iam::123:policy/demo"},
			wantErr: "confirmation required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			_, err := tt.handler(context.Background(), mcp.ToolRequest{Arguments: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
			if called {
				t.Fatalf("client should not be invoked")
			}
		})
	}
}

func TestIAMUpdateRoleNoUpdates(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	called := false
	svc := &Service{
		ctx: ctx,
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			called = true
			return nil, "us-east-1", nil
		},
	}
	_, err := svc.handleIAMUpdateRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName": "demo",
		"confirm":  true,
	}})
	if err == nil || !strings.Contains(err.Error(), "no updates specified") {
		t.Fatalf("expected no updates error, got %v", err)
	}
	if !called {
		t.Fatalf("expected iam client to be requested")
	}
}
