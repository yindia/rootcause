package awseks

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	extypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func TestEKSSummarizers(t *testing.T) {
	cluster := extypes.Cluster{
		Name:    aws.String("demo"),
		Logging: &extypes.Logging{ClusterLogging: []extypes.LogSetup{{Enabled: aws.Bool(true), Types: []extypes.LogType{extypes.LogTypeApi}}}},
	}
	clusterSummary := summarizeCluster(cluster)
	logging := clusterSummary["logging"].(map[string]any)
	if len(logging) == 0 {
		t.Fatalf("expected logging summary")
	}

	addon := extypes.Addon{
		AddonName: aws.String("vpc-cni"),
		Health: &extypes.AddonHealth{
			Issues: []extypes.AddonIssue{
				{Code: extypes.AddonIssueCodeConfigurationConflict, Message: aws.String("conflict"), ResourceIds: []string{"r1"}},
			},
		},
	}
	addonSummary := summarizeAddon(addon)
	health := addonSummary["health"].([]map[string]any)
	if len(health) != 1 {
		t.Fatalf("expected addon health summary")
	}
}
