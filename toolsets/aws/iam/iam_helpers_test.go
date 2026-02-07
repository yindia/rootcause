package awsiam

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func TestParseJSONOrString(t *testing.T) {
	value := `{"a":1}`
	parsed := parseJSONOrString(value)
	if m, ok := parsed.(map[string]any); !ok || m["a"] != float64(1) {
		t.Fatalf("unexpected parsed value: %#v", parsed)
	}
	raw := "not-json"
	parsed = parseJSONOrString(raw)
	if parsed.(string) != raw {
		t.Fatalf("expected raw string fallback")
	}
}

func TestDecodePolicyDocument(t *testing.T) {
	encoded := url.QueryEscape(`{"Statement":[]}`)
	decoded := decodePolicyDocument(encoded)
	if decoded == encoded {
		t.Fatalf("expected decoded value")
	}
}

func TestRequireConfirm(t *testing.T) {
	if err := requireConfirm(map[string]any{"confirm": true}); err != nil {
		t.Fatalf("expected confirm to pass: %v", err)
	}
	if err := requireConfirm(map[string]any{}); err == nil {
		t.Fatalf("expected confirm error")
	}
}

func TestTypeHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := toString(42); got != "42" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toInt(json.Number("5"), 1); got != 5 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toInt("bad", 7); got != 7 {
		t.Fatalf("expected fallback toInt")
	}
	if got := toBool(nil, true); got != true {
		t.Fatalf("expected fallback toBool")
	}
	if got := toBool(false, true); got != false {
		t.Fatalf("expected false toBool")
	}
	if got := regionOrDefault(""); got != "us-east-1" {
		t.Fatalf("expected default region")
	}
}

func TestSummarizers(t *testing.T) {
	created := time.Now()
	role := iamtypes.Role{
		RoleName:           aws.String("demo"),
		Arn:                aws.String("arn:aws:iam::123:role/demo"),
		Path:               aws.String("/"),
		RoleId:             aws.String("RID"),
		Description:        aws.String("test role"),
		CreateDate:         &created,
		MaxSessionDuration: aws.Int32(3600),
	}
	summary := summarizeRole(role)
	if summary["name"] != "demo" {
		t.Fatalf("unexpected role summary: %#v", summary)
	}
	profile := &iamtypes.InstanceProfile{
		InstanceProfileName: aws.String("profile"),
		Arn:                 aws.String("arn:aws:iam::123:instance-profile/profile"),
		Path:                aws.String("/"),
		CreateDate:          &created,
		Roles: []iamtypes.Role{
			{RoleName: aws.String("demo"), Arn: aws.String("arn:aws:iam::123:role/demo")},
		},
	}
	profileSummary := summarizeInstanceProfile(profile)
	if profileSummary["name"] != "profile" {
		t.Fatalf("unexpected instance profile summary: %#v", profileSummary)
	}
	policy := iamtypes.Policy{
		PolicyName:       aws.String("policy"),
		Arn:              aws.String("arn:aws:iam::123:policy/policy"),
		PolicyId:         aws.String("PID"),
		Path:             aws.String("/"),
		DefaultVersionId: aws.String("v1"),
		AttachmentCount:  aws.Int32(1),
		IsAttachable:     true,
		Description:      aws.String("desc"),
	}
	policySummary := summarizePolicy(policy)
	if policySummary["name"] != "policy" {
		t.Fatalf("unexpected policy summary: %#v", policySummary)
	}
}
