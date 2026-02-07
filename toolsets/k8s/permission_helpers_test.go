package k8s

import "testing"

func TestAssumePolicyMentionsServiceAccount(t *testing.T) {
	data := map[string]any{
		"assumeRolePolicy": map[string]any{
			"Statement": []any{map[string]any{"Principal": "system:serviceaccount:default:api"}},
		},
	}
	if !assumePolicyMentionsServiceAccount(data, "default", "api") {
		t.Fatalf("expected service account to be detected")
	}
	if assumePolicyMentionsServiceAccount(data, "default", "missing") {
		t.Fatalf("expected missing service account")
	}
}
