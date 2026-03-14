package k8s

import (
	"fmt"

	"rootcause/internal/kube"
	"rootcause/internal/render"
)

type cloudInfo struct {
	provider string
	reason   string
}

func detectCloud(clients *kube.Clients) cloudInfo {
	if clients == nil {
		return cloudInfo{provider: kube.CloudUnknown, reason: "missing kube clients"}
	}
	provider, reason := kube.DetectCloud(clients.RestConfig)
	return cloudInfo{provider: provider, reason: reason}
}

func addCloudEvidence(analysis *render.Analysis, info cloudInfo) {
	if analysis == nil {
		return
	}
	if info.provider == "" {
		return
	}
	analysis.AddEvidence("cloud", fmt.Sprintf("%s (%s)", info.provider, info.reason))
}

func addCloudHints(analysis *render.Analysis, provider, area string) {
	if analysis == nil {
		return
	}
	switch provider {
	case kube.CloudGCP:
		switch area {
		case "auth":
			analysis.AddNextCheck("GKE: verify Workload Identity bindings and IAM roles")
		case "network":
			analysis.AddNextCheck("GKE: verify GLBC/Ingress controller and firewall rules")
		case "storage":
			analysis.AddNextCheck("GKE: verify PD CSI driver and StorageClass provisioner")
		case "image":
			analysis.AddNextCheck("GKE: verify Artifact Registry/GCR access and imagePullSecrets")
		}
	case kube.CloudAzure:
		switch area {
		case "auth":
			analysis.AddNextCheck("AKS: verify managed identity/federated credentials and role assignments")
		case "network":
			analysis.AddNextCheck("AKS: verify AGIC/Ingress controller and NSG rules")
		case "storage":
			analysis.AddNextCheck("AKS: verify Azure Disk/File CSI drivers and StorageClass provisioner")
		case "image":
			analysis.AddNextCheck("AKS: verify ACR access and imagePullSecrets")
		}
	}
}

func isAWSCloud(provider string) bool {
	return provider == kube.CloudAWS
}
