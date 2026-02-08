package aws

import (
	"context"
	"os"
	"strings"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	sdkconfig "github.com/aws/aws-sdk-go-v2/config"
)

const defaultRegion = "us-east-1"

func ResolveRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	return region
}

func LoadConfig(ctx context.Context, region string) (sdkaws.Config, error) {
	loadOpts := []func(*sdkconfig.LoadOptions) error{}
	if profile := ResolveProfile(); profile != "" {
		loadOpts = append(loadOpts, sdkconfig.WithSharedConfigProfile(profile))
	}
	if region = ResolveRegion(region); region != "" {
		loadOpts = append(loadOpts, sdkconfig.WithRegion(region))
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

func ResolveProfile() string {
	profile := strings.TrimSpace(os.Getenv("AWS_PROFILE"))
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("AWS_DEFAULT_PROFILE"))
	}
	return profile
}
