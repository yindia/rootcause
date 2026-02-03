package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"rootcause/internal/mcp"
)

func (t *Toolset) handleExec(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	pod := toString(args["pod"])
	container := toString(args["container"])
	command := toStringSlice(args["command"])
	allowShell := false
	if val, ok := args["allowShell"].(bool); ok {
		allowShell = val
	}
	if namespace == "" || pod == "" || len(command) == 0 {
		return errorResult(errors.New("namespace, pod, and command are required")), errors.New("namespace, pod, and command are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	if !allowShell && commandIsShell(command) {
		return errorResult(errors.New("shell commands are not allowed")), errors.New("shell commands are not allowed")
	}

	stdout, stderr, err := t.execCommand(ctx, namespace, pod, container, command)
	if err != nil {
		return errorResult(err), err
	}

	return mcp.ToolResult{Data: map[string]any{"stdout": stdout, "stderr": stderr}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("pods/%s/%s", namespace, pod)}}}, nil
}

func (t *Toolset) execCommand(ctx context.Context, namespace, pod, container string, command []string) (string, string, error) {
	reqURL := t.ctx.Clients.Typed.CoreV1().RESTClient().Post().Resource("pods").Name(pod).Namespace(namespace).SubResource("exec")
	options := &corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
		Stdin:     false,
		TTY:       false,
	}
	reqURL.VersionedParams(options, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(t.ctx.Clients.RestConfig, "POST", reqURL.URL())
	if err != nil {
		return "", "", err
	}
	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr}); err != nil {
		return "", "", err
	}
	out := t.ctx.Redactor.RedactString(stdout.String())
	errOut := t.ctx.Redactor.RedactString(stderr.String())
	return out, errOut, nil
}

func commandIsShell(command []string) bool {
	if len(command) == 0 {
		return false
	}
	base := strings.ToLower(command[0])
	return strings.Contains(base, "sh") || strings.Contains(base, "bash") || strings.Contains(base, "zsh")
}
