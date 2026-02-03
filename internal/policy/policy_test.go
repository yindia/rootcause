package policy

import "testing"

func TestCheckNamespaceEnforcement(t *testing.T) {
	auth := &Authorizer{}
	user := User{ID: "ns-user", Role: RoleNamespace, AllowedNamespaces: []string{"team-a"}}

	if err := auth.CheckNamespace(user, "team-a", true); err != nil {
		t.Fatalf("expected namespace allowed, got error: %v", err)
	}
	if err := auth.CheckNamespace(user, "team-b", true); err == nil {
		t.Fatalf("expected namespace denied")
	}
	if err := auth.CheckNamespace(user, "", true); err == nil {
		t.Fatalf("expected namespace required error")
	}
	if err := auth.CheckNamespace(user, "", false); err == nil {
		t.Fatalf("expected cluster-scope denied for namespace role")
	}
}

func TestCheckNamespaceClusterRole(t *testing.T) {
	auth := &Authorizer{}
	user := User{ID: "cluster", Role: RoleCluster}

	if err := auth.CheckNamespace(user, "", false); err != nil {
		t.Fatalf("expected cluster-scope allowed, got %v", err)
	}
	if err := auth.CheckNamespace(user, "any", true); err != nil {
		t.Fatalf("expected namespaced allowed, got %v", err)
	}
}
