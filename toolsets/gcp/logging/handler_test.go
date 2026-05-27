package logging

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"rootcause/internal/mcp"
	"rootcause/toolsets/gcp/monitoring"
)

// fakeLoggingAPI captures calls and returns canned data.
type fakeLoggingAPI struct {
	entries    []map[string]any
	project    string
	err        error
	seenFilter string
	seenLimit  int
}

func (f *fakeLoggingAPI) FetchEntries(ctx context.Context, project, filter string, limit int) ([]map[string]any, string, error) {
	f.seenFilter = filter
	f.seenLimit = limit
	if f.err != nil {
		return nil, "", f.err
	}
	used := f.project
	if used == "" {
		used = project
	}
	return f.entries, used, nil
}

// fakeMetricsAPI provides RunMQL for error_timeline.
type fakeMetricsAPI struct {
	series    []map[string]any
	project   string
	err       error
	seenMQL   string
	seenLimit int
}

func (f *fakeMetricsAPI) RunMQL(ctx context.Context, project, mql string) ([]map[string]any, string, error) {
	f.seenMQL = mql
	if f.err != nil {
		return nil, "", f.err
	}
	used := f.project
	if used == "" {
		used = project
	}
	return f.series, used, nil
}
func (f *fakeMetricsAPI) ListDescriptors(ctx context.Context, project, filter string, limit int) ([]map[string]any, string, error) {
	return nil, project, nil
}
func (f *fakeMetricsAPI) ListServices(ctx context.Context, project string, limit int) ([]map[string]any, string, error) {
	return nil, project, nil
}
func (f *fakeMetricsAPI) ListSLOs(ctx context.Context, serviceName string, limit int) ([]map[string]any, error) {
	return nil, nil
}

func newService(api API, metricsAPI monitoring.API) *Service {
	return &Service{api: api, metricsAPI: metricsAPI}
}

func TestHandleQueryRequiresFilter(t *testing.T) {
	svc := newService(&fakeLoggingAPI{}, &fakeMetricsAPI{})
	_, err := svc.handleQuery(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}})
	if err == nil {
		t.Fatalf("expected error when filter missing")
	}
}

func TestHandleQuerySuccessWrapsFilterWithWindow(t *testing.T) {
	fake := &fakeLoggingAPI{
		entries: []map[string]any{{"severity": "ERROR"}},
		project: "p",
	}
	svc := newService(fake, &fakeMetricsAPI{})
	res, err := svc.handleQuery(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"filter":   `resource.type="k8s_container"`,
		"duration": "15m",
		"limit":    10,
	}})
	if err != nil {
		t.Fatalf("handleQuery: %v", err)
	}
	root := res.Data.(map[string]any)
	if root["count"].(int) != 1 {
		t.Errorf("expected count=1, got %v", root["count"])
	}
	if !strings.Contains(fake.seenFilter, `timestamp>=`) {
		t.Errorf("expected window clause appended, got %q", fake.seenFilter)
	}
	if fake.seenLimit != 10 {
		t.Errorf("expected limit forwarded, got %d", fake.seenLimit)
	}
}

func TestHandleWorkloadRequiresFields(t *testing.T) {
	svc := newService(&fakeLoggingAPI{}, &fakeMetricsAPI{})
	_, err := svc.handleWorkload(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"namespace": "x"}})
	if err == nil {
		t.Fatalf("expected error when workload missing")
	}
}

func TestHandleWorkloadBuildsFilter(t *testing.T) {
	fake := &fakeLoggingAPI{entries: []map[string]any{{"severity": "WARNING"}}, project: "p"}
	svc := newService(fake, &fakeMetricsAPI{})
	res, err := svc.handleWorkload(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"namespace": "payments",
		"workload":  "api",
		"severity":  "error",
	}})
	if err != nil {
		t.Fatalf("handleWorkload: %v", err)
	}
	root := res.Data.(map[string]any)
	if root["severity"] != "ERROR" {
		t.Errorf("expected severity to be normalized to ERROR, got %v", root["severity"])
	}
	for _, must := range []string{
		`namespace_name="payments"`,
		`pod_name:"api-"`,
		`severity>=ERROR`,
	} {
		if !strings.Contains(fake.seenFilter, must) {
			t.Errorf("expected filter to contain %q, got %q", must, fake.seenFilter)
		}
	}
}

func TestHandleErrorTimelineRequiresMetricsAPI(t *testing.T) {
	svc := newService(&fakeLoggingAPI{}, nil)
	_, err := svc.handleErrorTimeline(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"namespace": "p"}})
	if err == nil {
		t.Fatalf("expected error when metricsAPI is nil")
	}
}

func TestHandleErrorTimelineRequiresNamespace(t *testing.T) {
	svc := newService(&fakeLoggingAPI{}, &fakeMetricsAPI{})
	_, err := svc.handleErrorTimeline(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}})
	if err == nil {
		t.Fatalf("expected error when namespace missing")
	}
}

func TestHandleErrorTimelineUsesMQLAndBuckets(t *testing.T) {
	metrics := &fakeMetricsAPI{
		project: "obs-proj",
		series: []map[string]any{
			{
				"labelValues": []string{"ERROR"},
				"points": []map[string]any{
					{"start": isoNowOffset(-25), "end": isoNowOffset(-20), "value": float64(5)},
				},
			},
		},
	}
	svc := newService(&fakeLoggingAPI{}, metrics)
	res, err := svc.handleErrorTimeline(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"namespace":  "payments",
		"workload":   "api",
		"duration":   "1h",
		"bucketSize": "5m",
	}})
	if err != nil {
		t.Fatalf("handleErrorTimeline: %v", err)
	}
	root := res.Data.(map[string]any)
	if root["project"] != "obs-proj" {
		t.Errorf("expected project=obs-proj, got %v", root["project"])
	}
	if root["totalCount"].(int) != 5 {
		t.Errorf("expected totalCount=5, got %v", root["totalCount"])
	}
	if root["source"] != "logging.googleapis.com/log_entry_count" {
		t.Errorf("expected source label set, got %v", root["source"])
	}
	for _, must := range []string{
		"fetch k8s_container::logging.googleapis.com/log_entry_count",
		"resource.namespace_name == 'payments'",
		"metric.severity == 'ERROR'",
		"align delta(5m)",
		"group_by [metric.severity], sum(val())",
		"within 1h",
	} {
		if !strings.Contains(metrics.seenMQL, must) {
			t.Errorf("MQL missing %q\nactual: %s", must, metrics.seenMQL)
		}
	}
}

func TestHandleErrorTimelineHonorsResourceType(t *testing.T) {
	metrics := &fakeMetricsAPI{project: "obs-proj"}
	svc := newService(&fakeLoggingAPI{}, metrics)
	res, err := svc.handleErrorTimeline(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"namespace":    "payments",
		"resourceType": "generic_node",
	}})
	if err != nil {
		t.Fatalf("handleErrorTimeline: %v", err)
	}
	if !strings.Contains(metrics.seenMQL, "fetch generic_node::logging.googleapis.com/log_entry_count") {
		t.Errorf("expected generic_node in MQL, got: %s", metrics.seenMQL)
	}
	if res.Data.(map[string]any)["resourceType"] != "generic_node" {
		t.Errorf("expected resourceType echoed in output, got %v", res.Data.(map[string]any)["resourceType"])
	}
}

func TestHandleErrorTimelinePropagatesMQLError(t *testing.T) {
	metrics := &fakeMetricsAPI{err: errors.New("permission denied")}
	svc := newService(&fakeLoggingAPI{}, metrics)
	_, err := svc.handleErrorTimeline(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"namespace": "payments",
	}})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected MQL error to propagate, got %v", err)
	}
}

func TestHandleCorrelatedWithBundleDerivesFromBundle(t *testing.T) {
	fake := &fakeLoggingAPI{entries: []map[string]any{{"severity": "ERROR"}}, project: "p"}
	svc := newService(fake, &fakeMetricsAPI{})
	bundle := map[string]any{
		"namespace": "payments",
		"sections": map[string]any{
			"eventsTimeline": map[string]any{
				"timeline": []any{
					map[string]any{"time": "2026-05-23T09:00:00Z"},
					map[string]any{"time": "2026-05-23T10:00:00Z"},
				},
			},
		},
	}
	res, err := svc.handleCorrelatedWithBundle(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"bundle":   bundle,
		"workload": "api",
		"severity": "warning",
	}})
	if err != nil {
		t.Fatalf("handleCorrelatedWithBundle: %v", err)
	}
	root := res.Data.(map[string]any)
	if root["namespace"] != "payments" {
		t.Errorf("expected namespace derived from bundle, got %v", root["namespace"])
	}
	if root["startTime"] != "2026-05-23T09:00:00Z" {
		t.Errorf("expected startTime derived, got %v", root["startTime"])
	}
	if root["endTime"] != "2026-05-23T10:00:00Z" {
		t.Errorf("expected endTime derived, got %v", root["endTime"])
	}
	if !strings.Contains(fake.seenFilter, `timestamp>="2026-05-23T09:00:00Z"`) {
		t.Errorf("expected window in filter, got %q", fake.seenFilter)
	}
}

func TestHandleCorrelatedWithBundleRequiresNamespace(t *testing.T) {
	svc := newService(&fakeLoggingAPI{}, &fakeMetricsAPI{})
	_, err := svc.handleCorrelatedWithBundle(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"startTime": "2026-05-23T09:00:00Z",
		"endTime":   "2026-05-23T10:00:00Z",
	}})
	if err == nil {
		t.Fatalf("expected error when namespace missing")
	}
}

// isoNowOffset returns an RFC3339 timestamp N minutes from now (negative = past).
func isoNowOffset(minutes int) string {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339)
}
