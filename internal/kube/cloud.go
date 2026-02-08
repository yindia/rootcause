package kube

import (
	"strings"

	"k8s.io/client-go/rest"
)

const (
	CloudAWS     = "aws"
	CloudGCP     = "gcp"
	CloudAzure   = "azure"
	CloudUnknown = "unknown"
)

func DetectCloud(restConfig *rest.Config) (string, string) {
	if restConfig == nil {
		return CloudUnknown, "missing rest config"
	}
	host := strings.ToLower(restConfig.Host)
	switch {
	case strings.Contains(host, "eks.amazonaws.com") || (strings.Contains(host, ".eks.") && strings.Contains(host, "amazonaws.com")):
		return CloudAWS, "host match"
	case strings.Contains(host, "googleapis.com") || strings.Contains(host, "gke.") || strings.Contains(host, "gke"):
		return CloudGCP, "host match"
	case strings.Contains(host, "azmk8s.io"):
		return CloudAzure, "host match"
	}

	if restConfig.ExecProvider != nil {
		cmd := strings.ToLower(restConfig.ExecProvider.Command)
		switch {
		case strings.Contains(cmd, "aws") || strings.Contains(cmd, "aws-iam-authenticator"):
			return CloudAWS, "exec provider"
		case strings.Contains(cmd, "gcloud") || strings.Contains(cmd, "gke"):
			return CloudGCP, "exec provider"
		case strings.Contains(cmd, "kubelogin") || strings.Contains(cmd, "azure"):
			return CloudAzure, "exec provider"
		}
	}

	if restConfig.AuthProvider != nil {
		name := strings.ToLower(restConfig.AuthProvider.Name)
		switch {
		case strings.Contains(name, "aws"):
			return CloudAWS, "auth provider"
		case strings.Contains(name, "gcp"):
			return CloudGCP, "auth provider"
		case strings.Contains(name, "azure"):
			return CloudAzure, "auth provider"
		}
	}

	return CloudUnknown, "no match"
}
