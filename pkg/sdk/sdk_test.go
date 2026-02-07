package sdk

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/restmapper"
)

func TestResolveResourceWrapper(t *testing.T) {
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "deployments", Kind: "Deployment", Namespaced: true}},
			},
		},
	})
	gvr, namespaced, err := ResolveResource(mapper, "apps/v1", "Deployment", "")
	if err != nil {
		t.Fatalf("resolve resource: %v", err)
	}
	if gvr.Resource != "deployments" || !namespaced {
		t.Fatalf("unexpected gvr: %#v namespaced=%v", gvr, namespaced)
	}
}

func TestRegisterAndListToolsets(t *testing.T) {
	id := fmt.Sprintf("sdk-test-%d", time.Now().UnixNano())
	err := RegisterToolset(id, func() Toolset { return nil })
	if err != nil {
		t.Fatalf("register toolset: %v", err)
	}
	toolsets := RegisteredToolsets()
	found := false
	for _, name := range toolsets {
		if name == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected toolset id %s in list", id)
	}
}
