package aws

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	if region := ResolveRegion(""); region != "us-west-2" {
		t.Fatalf("expected env region, got %q", region)
	}
	if region := ResolveRegion("eu-central-1"); region != "eu-central-1" {
		t.Fatalf("expected explicit region, got %q", region)
	}
}

func TestResolveProfile(t *testing.T) {
	t.Setenv("AWS_PROFILE", "dev")
	if profile := ResolveProfile(); profile != "dev" {
		t.Fatalf("expected profile, got %q", profile)
	}
}

func TestLoadConfigDefaultRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	dir := t.TempDir()
	credentials := `[default]
aws_access_key_id = test
aws_secret_access_key = secret
`
	if err := os.WriteFile(filepath.Join(dir, "credentials"), []byte(credentials), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("[default]\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	cfg, err := LoadConfig(context.Background(), "")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Region != defaultRegion {
		t.Fatalf("expected default region, got %q", cfg.Region)
	}
}

func TestLoadConfigUsesRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	cfg, err := LoadConfig(context.Background(), "ap-south-1")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Region != "ap-south-1" {
		t.Fatalf("expected region ap-south-1, got %q", cfg.Region)
	}
}
