package policy

import (
	"errors"
	"strings"
)

type Role string

const (
	RoleNamespace Role = "namespace"
	RoleCluster   Role = "cluster"
)

type User struct {
	ID                string
	Role              Role
	AllowedNamespaces []string
	AllowedToolsets   []string
	AllowedTools      []string
}

type Authorizer struct {
}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

func (a *Authorizer) Authenticate(apiKey string) (User, error) {
	_ = apiKey
	return User{ID: "local", Role: RoleCluster}, nil
}

func (a *Authorizer) AuthorizeTool(user User, toolsetID, toolName string) error {
	_ = user
	_ = toolsetID
	_ = toolName
	return nil
}

func (a *Authorizer) CheckNamespace(user User, namespace string, namespaced bool) error {
	if user.Role == RoleCluster {
		return nil
	}
	if !namespaced {
		return errors.New("cluster-scoped access denied for namespace role")
	}
	if namespace == "" {
		return errors.New("namespace required for namespace role")
	}
	for _, allowed := range user.AllowedNamespaces {
		if allowed == namespace {
			return nil
		}
	}
	return errors.New("namespace not allowed")
}

func (a *Authorizer) FilterNamespaces(user User, namespaces []string) []string {
	if user.Role == RoleCluster {
		return namespaces
	}
	allowed := map[string]struct{}{}
	for _, namespace := range user.AllowedNamespaces {
		allowed[namespace] = struct{}{}
	}
	var filtered []string
	for _, namespace := range namespaces {
		if _, ok := allowed[namespace]; ok {
			filtered = append(filtered, namespace)
		}
	}
	return filtered
}

func HasNamespaceInToolName(toolName string) bool {
	return strings.Contains(toolName, "namespace")
}
