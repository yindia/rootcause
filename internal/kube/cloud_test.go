package kube

import (
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestDetectCloudFromHost(t *testing.T) {
	cloud, _ := DetectCloud(&rest.Config{Host: "https://ABC.eks.amazonaws.com"})
	if cloud != CloudAWS {
		t.Fatalf("expected aws, got %s", cloud)
	}
	cloud, _ = DetectCloud(&rest.Config{Host: "https://cluster.gke.us-central1.googleapis.com"})
	if cloud != CloudGCP {
		t.Fatalf("expected gcp, got %s", cloud)
	}
	cloud, _ = DetectCloud(&rest.Config{Host: "https://demo.azmk8s.io"})
	if cloud != CloudAzure {
		t.Fatalf("expected azure, got %s", cloud)
	}
}

func TestDetectCloudFromExecProvider(t *testing.T) {
	cloud, _ := DetectCloud(&rest.Config{ExecProvider: &api.ExecConfig{Command: "aws"}})
	if cloud != CloudAWS {
		t.Fatalf("expected aws, got %s", cloud)
	}
	cloud, _ = DetectCloud(&rest.Config{ExecProvider: &api.ExecConfig{Command: "gke-gcloud-auth-plugin"}})
	if cloud != CloudGCP {
		t.Fatalf("expected gcp, got %s", cloud)
	}
}

func TestDetectCloudFromAuthProvider(t *testing.T) {
	cloud, _ := DetectCloud(&rest.Config{AuthProvider: &api.AuthProviderConfig{Name: "gcp"}})
	if cloud != CloudGCP {
		t.Fatalf("expected gcp, got %s", cloud)
	}
}

func TestDetectCloudUnknown(t *testing.T) {
	cloud, _ := DetectCloud(nil)
	if cloud != CloudUnknown {
		t.Fatalf("expected unknown, got %s", cloud)
	}
	cloud, _ = DetectCloud(&rest.Config{Host: "https://example.com"})
	if cloud != CloudUnknown {
		t.Fatalf("expected unknown, got %s", cloud)
	}
}
