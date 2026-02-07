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

func TestFilterNamespaces(t *testing.T) {
	auth := &Authorizer{}
	user := User{ID: "ns-user", Role: RoleNamespace, AllowedNamespaces: []string{"a", "b"}}
	filtered := auth.FilterNamespaces(user, []string{"a", "c", "b"})
	if len(filtered) != 2 || filtered[0] != "a" || filtered[1] != "b" {
		t.Fatalf("unexpected filtered list: %#v", filtered)
	}
}

func TestHasNamespaceInToolName(t *testing.T) {
	if !HasNamespaceInToolName("k8s.namespace") {
		t.Fatalf("expected namespace token")
	}
	if HasNamespaceInToolName("k8s.get") {
		t.Fatalf("did not expect namespace token")
	}
}

func TestFilterNamespacesClusterRole(t *testing.T) {
	auth := &Authorizer{}
	user := User{ID: "cluster", Role: RoleCluster}
	filtered := auth.FilterNamespaces(user, []string{"a", "b"})
	if len(filtered) != 2 {
		t.Fatalf("expected all namespaces, got %#v", filtered)
	}
}

func TestNewAuthorizer(t *testing.T) {
	auth := NewAuthorizer()
	if auth == nil {
		t.Fatalf("expected authorizer instance")
	}
}

func TestAuthenticateDefaultUser(t *testing.T) {
	auth := NewAuthorizer()
	user, err := auth.Authenticate("ignored")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if user.ID != "local" || user.Role != RoleCluster {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestAuthorizeToolNoop(t *testing.T) {
	auth := NewAuthorizer()
	user := User{ID: "local", Role: RoleCluster}
	if err := auth.AuthorizeTool(user, "k8s", "k8s.get"); err != nil {
		t.Fatalf("expected authorize to succeed, got %v", err)
	}
}
