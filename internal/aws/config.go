package aws

import (
	"context"
	"os"
	"strings"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	sdkconfig "github.com/aws/aws-sdk-go-v2/config"
)

const defaultRegion = "us-east-1"

// ResolveRegion returns the explicit region when non-empty, then
// AWS_REGION env, then AWS_DEFAULT_REGION env, else "". For config-file
// fallback use ResolveRegionWithConfig.
func ResolveRegion(region string) string {
	return ResolveRegionWithConfig(region, "")
}

// ResolveRegionWithConfig adds a [aws].region config fallback to the
// resolution chain. Order: explicit > AWS_REGION env > AWS_DEFAULT_REGION
// env > cfgRegion (config file) > "".
func ResolveRegionWithConfig(region, cfgRegion string) string {
	region = strings.TrimSpace(region)
	if region != "" {
		return region
	}
	if v := strings.TrimSpace(os.Getenv("AWS_REGION")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION")); v != "" {
		return v
	}
	return strings.TrimSpace(cfgRegion)
}

// LoadConfig builds an AWS SDK config honoring the standard discovery chain.
// Use LoadConfigWithDefaults to thread [aws] config-file defaults
// (region/profile/credentials_file) through the SDK load options.
func LoadConfig(ctx context.Context, region string) (sdkaws.Config, error) {
	return LoadConfigWithDefaults(ctx, region, "", "")
}

// LoadConfigWithDefaults applies the resolution chains for region and profile,
// and optionally adds an explicit shared-credentials file path (typically
// [aws].credentials_file).
func LoadConfigWithDefaults(ctx context.Context, region, cfgRegion, cfgProfile string) (sdkaws.Config, error) {
	return LoadConfigWithSecrets(ctx, region, cfgRegion, cfgProfile, "")
}

// LoadConfigWithSecrets is the fullest form of the loader: it accepts the
// config-file values for region, profile, and shared credentials file.
func LoadConfigWithSecrets(ctx context.Context, region, cfgRegion, cfgProfile, cfgCredentialsFile string) (sdkaws.Config, error) {
	loadOpts := []func(*sdkconfig.LoadOptions) error{}
	if profile := ResolveProfileWithConfig(cfgProfile); profile != "" {
		loadOpts = append(loadOpts, sdkconfig.WithSharedConfigProfile(profile))
	}
	if r := ResolveRegionWithConfig(region, cfgRegion); r != "" {
		loadOpts = append(loadOpts, sdkconfig.WithRegion(r))
	}
	if cf := strings.TrimSpace(cfgCredentialsFile); cf != "" {
		loadOpts = append(loadOpts, sdkconfig.WithSharedCredentialsFiles([]string{cf}))
	}
	cfg, err := sdkconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.Region) == "" {
		cfg.Region = defaultRegion
	}
	return cfg, nil
}

// ResolveProfile returns AWS_PROFILE env, else AWS_DEFAULT_PROFILE env, else
// "". Use ResolveProfileWithConfig to add a [aws].profile config fallback.
func ResolveProfile() string {
	return ResolveProfileWithConfig("")
}

// ResolveProfileWithConfig adds a [aws].profile config fallback after env.
func ResolveProfileWithConfig(cfgProfile string) string {
	if v := strings.TrimSpace(os.Getenv("AWS_PROFILE")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("AWS_DEFAULT_PROFILE")); v != "" {
		return v
	}
	return strings.TrimSpace(cfgProfile)
}
