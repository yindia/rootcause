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

func TestResolveRegionWithConfigPrecedence(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("AWS_DEFAULT_REGION", "")
	if got := ResolveRegionWithConfig("explicit-1", "cfg-region"); got != "explicit-1" {
		t.Errorf("explicit should win, got %q", got)
	}
	if got := ResolveRegionWithConfig("", "cfg-region"); got != "us-west-2" {
		t.Errorf("env should beat config, got %q", got)
	}
	t.Setenv("AWS_REGION", "")
	if got := ResolveRegionWithConfig("", "cfg-region"); got != "cfg-region" {
		t.Errorf("config should win when others empty, got %q", got)
	}
	if got := ResolveRegionWithConfig("", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveProfileWithConfigPrecedence(t *testing.T) {
	t.Setenv("AWS_PROFILE", "env-prof")
	t.Setenv("AWS_DEFAULT_PROFILE", "")
	if got := ResolveProfileWithConfig("cfg-prof"); got != "env-prof" {
		t.Errorf("env should win, got %q", got)
	}
	t.Setenv("AWS_PROFILE", "")
	if got := ResolveProfileWithConfig("cfg-prof"); got != "cfg-prof" {
		t.Errorf("config should win when env empty, got %q", got)
	}
}

func TestLoadConfigWithSecretsAppliesCredentialsFile(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_PROFILE", "")
	dir := t.TempDir()
	credentialsPath := filepath.Join(dir, "creds")
	content := `[acme]
aws_access_key_id = AKIATEST
aws_secret_access_key = secrettest
`
	if err := os.WriteFile(credentialsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	cfg, err := LoadConfigWithSecrets(context.Background(), "us-east-1", "", "acme", credentialsPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	creds, err := cfg.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("retrieve creds: %v", err)
	}
	if creds.AccessKeyID != "AKIATEST" {
		t.Errorf("expected acme profile creds from [aws].credentials_file, got AccessKeyID=%q", creds.AccessKeyID)
	}
}
