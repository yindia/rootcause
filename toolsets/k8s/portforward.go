package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/apimachinery/pkg/util/httpstream"

	"rootcause/internal/mcp"
)

type portForwarder interface {
	ForwardPorts() error
	GetPorts() ([]portforward.ForwardedPort, error)
}

var spdyRoundTripperFor = spdy.RoundTripperFor

var newPortForwarder = func(dialer httpstream.Dialer, addresses []string, ports []string, stopChan, readyChan chan struct{}, out, errOut io.Writer) (portForwarder, error) {
	return portforward.NewOnAddresses(dialer, addresses, ports, stopChan, readyChan, out, errOut)
}

func (t *Toolset) handlePortForward(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	pod := toString(args["pod"])
	service := toString(args["service"])
	ports := toStringSlice(args["ports"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if pod == "" && service == "" {
		return errorResult(errors.New("pod or service is required")), errors.New("pod or service is required")
	}
	if len(ports) == 0 {
		return errorResult(errors.New("ports are required")), errors.New("ports are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	if pod == "" {
		resolved, err := t.resolvePodForService(ctx, namespace, service)
		if err != nil {
			return errorResult(err), err
		}
		pod = resolved
	}

	var portMeta []servicePortMapping
	if service != "" {
		resolvedPorts, mappings, err := t.resolveServicePortMappings(ctx, namespace, service, pod, ports)
		if err != nil {
			return errorResult(err), err
		}
		ports = resolvedPorts
		portMeta = mappings
	}

	localAddress := toString(args["localAddress"])
	if localAddress == "" {
		localAddress = "127.0.0.1"
	}
	var durationSeconds int64 = 60
	if val, ok := args["durationSeconds"].(float64); ok {
		durationSeconds = int64(val)
	}
	if durationSeconds <= 0 {
		durationSeconds = 60
	}

	restClient := t.ctx.Clients.Typed.CoreV1().RESTClient()
	if restClient == nil {
		return errorResult(errors.New("rest client not available")), errors.New("rest client not available")
	}
	if client, ok := restClient.(*rest.RESTClient); ok && client == nil {
		restClient = nil
	}
	if restClient == nil {
		if t.ctx.Clients.RestConfig == nil {
			return errorResult(errors.New("rest client not available")), errors.New("rest client not available")
		}
		cfg := rest.CopyConfig(t.ctx.Clients.RestConfig)
		if cfg.GroupVersion == nil {
			cfg.GroupVersion = &schema.GroupVersion{Version: "v1"}
		}
		if cfg.APIPath == "" {
			cfg.APIPath = "/api"
		}
		if cfg.NegotiatedSerializer == nil {
			cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		}
		client, err := rest.RESTClientFor(cfg)
		if err != nil {
			return errorResult(err), err
		}
		restClient = client
	}
	reqURL := restClient.Post().Resource("pods").Namespace(namespace).Name(pod).SubResource("portforward")
	transport, upgrader, err := spdyRoundTripperFor(t.ctx.Clients.RestConfig)
	if err != nil {
		return errorResult(err), err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL.URL())

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	pf, err := newPortForwarder(dialer, []string{localAddress}, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return errorResult(err), err
	}

	go func() {
		_ = pf.ForwardPorts()
	}()

	select {
	case <-readyChan:
	case <-ctx.Done():
		close(stopChan)
		return errorResult(ctx.Err()), ctx.Err()
	case <-time.After(10 * time.Second):
		close(stopChan)
		return errorResult(errors.New("port-forward ready timeout")), errors.New("port-forward ready timeout")
	}

	forwarded, err := pf.GetPorts()
	if err != nil {
		close(stopChan)
		return errorResult(err), err
	}

	go func() {
		timer := time.NewTimer(time.Duration(durationSeconds) * time.Second)
		defer timer.Stop()
		<-timer.C
		close(stopChan)
	}()

	var mapped []map[string]any
	if len(portMeta) > 0 {
		for i, port := range forwarded {
			entry := map[string]any{
				"local":  port.Local,
				"remote": port.Remote,
			}
			if i < len(portMeta) {
				entry["servicePort"] = portMeta[i].ServicePort
				entry["targetPort"] = port.Remote
				if portMeta[i].ServicePortName != "" {
					entry["servicePortName"] = portMeta[i].ServicePortName
				}
			}
			mapped = append(mapped, entry)
		}
	} else {
		for _, port := range forwarded {
			mapped = append(mapped, map[string]any{
				"local":  port.Local,
				"remote": port.Remote,
			})
		}
	}

	data := map[string]any{
		"pod":             pod,
		"namespace":       namespace,
		"localAddress":    localAddress,
		"ports":           mapped,
		"durationSeconds": durationSeconds,
	}
	if service != "" {
		data["service"] = service
	}
	if errOut.Len() > 0 {
		data["warnings"] = errOut.String()
	}
	return mcp.ToolResult{Data: data, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("pods/%s/%s", namespace, pod)}}}, nil
}

type servicePortMapping struct {
	ServicePort     int32
	TargetPort      int32
	ServicePortName string
	LocalPort       int
}

func (t *Toolset) resolveServicePortMappings(ctx context.Context, namespace, service, pod string, ports []string) ([]string, []servicePortMapping, error) {
	svc, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	portByName := map[string]corev1.ServicePort{}
	portByNumber := map[int32]corev1.ServicePort{}
	for _, port := range svc.Spec.Ports {
		portByNumber[port.Port] = port
		if port.Name != "" {
			portByName[port.Name] = port
		}
	}

	podObj, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	var resolved []string
	var mappings []servicePortMapping
	for _, spec := range ports {
		localStr, remoteStr, err := parsePortSpec(spec)
		if err != nil {
			return nil, nil, err
		}
		localPort, err := strconv.Atoi(localStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid local port %q", localStr)
		}

		var svcPort corev1.ServicePort
		if remotePort, ok := parsePortNumber(remoteStr); ok {
			match, ok := portByNumber[int32(remotePort)]
			if !ok {
				return nil, nil, fmt.Errorf("service port %d not found on service %s", remotePort, service)
			}
			svcPort = match
		} else {
			match, ok := portByName[remoteStr]
			if !ok {
				return nil, nil, fmt.Errorf("service port name %q not found on service %s", remoteStr, service)
			}
			svcPort = match
		}

		targetPort, err := resolveTargetPort(svcPort, podObj)
		if err != nil {
			return nil, nil, err
		}
		resolved = append(resolved, fmt.Sprintf("%d:%d", localPort, targetPort))
		mappings = append(mappings, servicePortMapping{
			ServicePort:     svcPort.Port,
			TargetPort:      targetPort,
			ServicePortName: svcPort.Name,
			LocalPort:       localPort,
		})
	}
	return resolved, mappings, nil
}

func parsePortSpec(spec string) (string, string, error) {
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return "", "", errors.New("empty port spec")
	}
	parts := strings.Split(trimmed, ":")
	switch len(parts) {
	case 1:
		return parts[0], parts[0], nil
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid port mapping %q", spec)
		}
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("invalid port mapping %q", spec)
	}
}

func parsePortNumber(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
	}
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return num, true
}

func resolveTargetPort(port corev1.ServicePort, pod *corev1.Pod) (int32, error) {
	switch port.TargetPort.Type {
	case intstr.Int:
		if port.TargetPort.IntVal != 0 {
			return port.TargetPort.IntVal, nil
		}
	case intstr.String:
		if port.TargetPort.StrVal != "" {
			return resolveNamedPort(port.TargetPort.StrVal, pod)
		}
	}
	if port.Port != 0 {
		return port.Port, nil
	}
	return 0, errors.New("unable to resolve service targetPort")
}

func resolveNamedPort(name string, pod *corev1.Pod) (int32, error) {
	if pod == nil {
		return 0, fmt.Errorf("pod required to resolve targetPort %q", name)
	}
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == name {
				return port.ContainerPort, nil
			}
		}
	}
	return 0, fmt.Errorf("targetPort %q not found in pod %s", name, pod.Name)
}

func (t *Toolset) resolvePodForService(ctx context.Context, namespace, service string) (string, error) {
	if service == "" {
		return "", errors.New("service is required")
	}
	endpoints, err := t.ctx.Clients.Typed.CoreV1().Endpoints(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
				return addr.TargetRef.Name, nil
			}
		}
	}
	return "", errors.New("no pod found for service endpoints")
}
