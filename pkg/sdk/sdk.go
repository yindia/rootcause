package sdk

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

// Core toolset interfaces and types.
type Toolset = mcp.Toolset

type ToolsetContext = mcp.ToolsetContext

type ToolSpec = mcp.ToolSpec

type ToolHandler = mcp.ToolHandler

type ToolSafety = mcp.ToolSafety

type ToolRequest = mcp.ToolRequest

type ToolResult = mcp.ToolResult

type ToolMetadata = mcp.ToolMetadata

type Registry = mcp.Registry

const (
	SafetyReadOnly    = mcp.SafetyReadOnly
	SafetyWrite       = mcp.SafetyWrite
	SafetyRiskyWrite  = mcp.SafetyRiskyWrite
	SafetyDestructive = mcp.SafetyDestructive
)

// Toolset registration for plugin discovery.
func RegisterToolset(id string, factory mcp.ToolsetFactory) error {
	return mcp.RegisterToolset(id, factory)
}

func MustRegisterToolset(id string, factory mcp.ToolsetFactory) {
	mcp.MustRegisterToolset(id, factory)
}

func RegisteredToolsets() []string {
	return mcp.RegisteredToolsets()
}

// Shared services and invoker.
type ServiceRegistry = mcp.ServiceRegistry

type ToolInvoker = mcp.ToolInvoker

// Kubernetes helpers.
type Clients = kube.Clients

type Collector = evidence.Collector

type Renderer = render.Renderer

type Redactor = redact.Redactor

func ResolveResource(mapper meta.RESTMapper, apiVersion, kind, resource string) (schema.GroupVersionResource, bool, error) {
	return kube.ResolveResource(mapper, apiVersion, kind, resource)
}

func ResolveResourceBestEffort(mapper meta.RESTMapper, discoveryClient discovery.DiscoveryInterface, apiVersion, kind, resource, group string) (schema.GroupVersionResource, bool, error) {
	return kube.ResolveResourceBestEffort(mapper, discoveryClient, apiVersion, kind, resource, group)
}

func DescribeAnalysis(ctx context.Context, collector evidence.Collector, redactor *redact.Redactor, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) render.Analysis {
	return render.DescribeAnalysis(ctx, collector, redactor, gvr, obj)
}

// Policy helpers.
type User = policy.User

type Role = policy.Role

const (
	RoleNamespace = policy.RoleNamespace
	RoleCluster   = policy.RoleCluster
)
