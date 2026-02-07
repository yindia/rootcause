package aws

import "testing"

func TestClientCacheKey(t *testing.T) {
	t.Setenv("AWS_PROFILE", "dev")
	t.Setenv("AWS_REGION", "us-west-2")
	toolset := &Toolset{}
	key := toolset.clientCacheKey("")
	if key != "dev|us-west-2" {
		t.Fatalf("expected profile+region key, got %q", key)
	}

	t.Setenv("AWS_PROFILE", "")
	key = toolset.clientCacheKey("")
	if key != "us-west-2" {
		t.Fatalf("expected region key, got %q", key)
	}

	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	key = toolset.clientCacheKey("")
	if key != "default" {
		t.Fatalf("expected default key, got %q", key)
	}
}
