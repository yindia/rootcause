package logging

import (
	"strings"
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"2026-05-23T10:00:00Z", "2026-05-23T10:00:00Z", false},
		{"2026-05-23T10:00:00.123456789Z", "2026-05-23T10:00:00Z", false},
		{"not-a-time", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := parseTime(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parseTime(%q) expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTime(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got.UTC().Format(time.RFC3339) != c.want {
			t.Errorf("parseTime(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestResolveTimeRange(t *testing.T) {
	start, end, err := resolveTimeRange("2026-05-23T09:00:00Z", "2026-05-23T10:00:00Z", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end.Sub(start) != time.Hour {
		t.Fatalf("expected 1h window, got %v", end.Sub(start))
	}

	start, end, err = resolveTimeRange("", "2026-05-23T10:00:00Z", 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end.Sub(start) != 30*time.Minute {
		t.Fatalf("expected 30m fallback, got %v", end.Sub(start))
	}

	if _, _, err := resolveTimeRange("2026-05-23T11:00:00Z", "2026-05-23T10:00:00Z", time.Hour); err == nil {
		t.Fatalf("expected error for start >= end")
	}
	if _, _, err := resolveTimeRange("garbage", "", time.Hour); err == nil {
		t.Fatalf("expected error for bad startTime")
	}
}

func TestBundleWindowFilter(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-05-23T09:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-05-23T10:00:00Z")

	got := bundleWindowFilter("payments", "api", "ERROR", start, end)
	for _, must := range []string{
		`namespace_name="payments"`,
		`pod_name:"api-"`,
		`severity>=ERROR`,
		`timestamp>="2026-05-23T09:00:00Z"`,
		`timestamp<="2026-05-23T10:00:00Z"`,
	} {
		if !strings.Contains(got, must) {
			t.Errorf("expected filter to contain %q\nactual: %s", must, got)
		}
	}

	got = bundleWindowFilter("payments", "", "WARNING", start, end)
	if strings.Contains(got, "pod_name") {
		t.Errorf("expected no pod_name when workload empty, got: %s", got)
	}
}

func TestExtractBundleWindowScansEventsAndHelm(t *testing.T) {
	bundle := map[string]any{
		"sections": map[string]any{
			"eventsTimeline": map[string]any{
				"timeline": []any{
					map[string]any{"time": "2026-05-23T09:05:00Z"},
					map[string]any{"time": "2026-05-23T09:30:00Z"},
				},
			},
			"helmReleases": map[string]any{
				"releases": []any{
					map[string]any{"updated": "2026-05-23T08:50:00Z"},
					map[string]any{"updated": "2026-05-23T10:15:00Z"},
				},
			},
		},
	}
	start, end := extractBundleWindow(bundle)
	if start.Format(time.RFC3339) != "2026-05-23T08:50:00Z" {
		t.Errorf("expected earliest = helm release, got %v", start)
	}
	if end.Format(time.RFC3339) != "2026-05-23T10:15:00Z" {
		t.Errorf("expected latest = later helm release, got %v", end)
	}
}

func TestExtractBundleWindowPrefersTopLevelTimes(t *testing.T) {
	bundle := map[string]any{
		"startTime": "2026-05-23T08:00:00Z",
		"endTime":   "2026-05-23T11:00:00Z",
		"sections": map[string]any{
			"eventsTimeline": map[string]any{
				"timeline": []any{
					map[string]any{"time": "2026-05-23T09:00:00Z"},
				},
			},
		},
	}
	start, end := extractBundleWindow(bundle)
	if start.Format(time.RFC3339) != "2026-05-23T08:00:00Z" {
		t.Errorf("expected earliest = top-level startTime, got %v", start)
	}
	if end.Format(time.RFC3339) != "2026-05-23T11:00:00Z" {
		t.Errorf("expected latest = top-level endTime, got %v", end)
	}
}

func TestExtractBundleWindowEmptyBundle(t *testing.T) {
	start, end := extractBundleWindow(map[string]any{})
	if !start.IsZero() || !end.IsZero() {
		t.Errorf("expected zero times for empty bundle, got %v, %v", start, end)
	}
}

func TestWithinWindowAddsTimestampClause(t *testing.T) {
	got := withinWindow(`resource.type="k8s_container"`, 30*time.Minute)
	if !strings.Contains(got, `timestamp>=`) {
		t.Errorf("expected timestamp clause, got: %s", got)
	}
	if !strings.HasPrefix(got, `(resource.type="k8s_container") AND timestamp>="`) {
		t.Errorf("expected wrapped filter, got: %s", got)
	}
}

func TestNonEmptyList(t *testing.T) {
	got := nonEmptyList("a", "", "b", "  ", "c")
	if len(got) != 3 {
		t.Fatalf("expected 3 non-empty, got %v", got)
	}
	if got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("expected sorted [a b c], got %v", got)
	}
}

func TestWorkloadFilterHasAllParts(t *testing.T) {
	got := workloadFilter("payments", "api", "ERROR", 30*time.Minute)
	for _, must := range []string{
		`namespace_name="payments"`,
		`pod_name:"api-"`,
		`severity>=ERROR`,
		`timestamp>=`,
	} {
		if !strings.Contains(got, must) {
			t.Errorf("workloadFilter missing %q in %s", must, got)
		}
	}
}

func TestSeveritiesAtOrAbove(t *testing.T) {
	got, err := severitiesAtOrAbove("ERROR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"ERROR", "CRITICAL", "ALERT", "EMERGENCY"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i, s := range want {
		if got[i] != s {
			t.Errorf("severitiesAtOrAbove[%d] = %s, want %s", i, got[i], s)
		}
	}

	if _, err := severitiesAtOrAbove("WAT"); err == nil {
		t.Fatalf("expected error for invalid severity")
	}
}

func TestErrorTimelineMQLShape(t *testing.T) {
	mql := errorTimelineMQL("k8s_container", "payments", "api", []string{"ERROR", "CRITICAL"}, time.Hour, 5*time.Minute)
	for _, must := range []string{
		"fetch k8s_container::logging.googleapis.com/log_entry_count",
		"resource.namespace_name == 'payments'",
		"resource.pod_name =~ 'api-.*'",
		"metric.severity == 'ERROR'",
		"metric.severity == 'CRITICAL'",
		"align delta(5m)",
		"every 5m",
		"group_by [metric.severity], sum(val())",
		"within 1h",
	} {
		if !strings.Contains(mql, must) {
			t.Errorf("MQL missing %q\nactual: %s", must, mql)
		}
	}

	mql = errorTimelineMQL("k8s_container", "payments", "", []string{"ERROR"}, 30*time.Minute, time.Minute)
	if strings.Contains(mql, "pod_name") {
		t.Errorf("expected no pod_name filter when workload empty: %s", mql)
	}

	// Custom resource type is honored.
	mql = errorTimelineMQL("generic_node", "payments", "api", []string{"ERROR"}, time.Hour, time.Minute)
	if !strings.Contains(mql, "fetch generic_node::logging.googleapis.com/log_entry_count") {
		t.Errorf("expected generic_node resource in MQL, got: %s", mql)
	}

	// Injection attempt falls back to the safe default.
	mql = errorTimelineMQL("k8s_container | delete", "payments", "api", []string{"ERROR"}, time.Hour, time.Minute)
	if !strings.Contains(mql, "fetch k8s_container::") || strings.Contains(mql, "delete") {
		t.Errorf("expected injection to fall back to k8s_container, got: %s", mql)
	}
}

func TestBucketizeFromTimeSeries(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-05-23T09:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-05-23T10:00:00Z")
	series := []map[string]any{
		{
			"labelValues": []string{"ERROR"},
			"points": []map[string]any{
				{"start": "2026-05-23T09:05:00Z", "end": "2026-05-23T09:10:00Z", "value": float64(7)},
				{"start": "2026-05-23T09:40:00Z", "end": "2026-05-23T09:45:00Z", "value": float64(3)},
			},
		},
		{
			"labelValues": []string{"WARNING"},
			"points": []map[string]any{
				{"start": "2026-05-23T09:05:00Z", "end": "2026-05-23T09:10:00Z", "value": float64(2)},
			},
		},
	}
	buckets, total, severityTotals := bucketizeFromTimeSeries(series, start, end, 5*time.Minute)
	if len(buckets) != 12 {
		t.Fatalf("expected 12 buckets, got %d", len(buckets))
	}
	if total != 12 {
		t.Errorf("expected total 12, got %d", total)
	}
	if severityTotals["ERROR"] != 10 {
		t.Errorf("expected 10 ERROR, got %d", severityTotals["ERROR"])
	}
	if severityTotals["WARNING"] != 2 {
		t.Errorf("expected 2 WARNING, got %d", severityTotals["WARNING"])
	}

	// Verify which bucket got the first count (09:05 -> index 1, 9:05-9:10).
	if c := buckets[1]["count"].(int); c != 9 {
		t.Errorf("expected bucket[1] count=9 (7 ERROR + 2 WARNING), got %d", c)
	}
	breakdown := buckets[1]["severityBreakdown"].(map[string]int)
	if breakdown["ERROR"] != 7 || breakdown["WARNING"] != 2 {
		t.Errorf("unexpected breakdown for bucket[1]: %v", breakdown)
	}
}

func TestBucketizeFromTimeSeriesEmpty(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-05-23T09:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-05-23T10:00:00Z")
	buckets, total, severityTotals := bucketizeFromTimeSeries(nil, start, end, 5*time.Minute)
	if len(buckets) != 12 {
		t.Fatalf("expected 12 zero-buckets, got %d", len(buckets))
	}
	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if len(severityTotals) != 0 {
		t.Errorf("expected empty severityTotals, got %v", severityTotals)
	}
}

func TestNumericValue(t *testing.T) {
	if numericValue(float64(3.5)) != 3.5 {
		t.Errorf("float64 conversion failed")
	}
	if numericValue(int(7)) != 7 {
		t.Errorf("int conversion failed")
	}
	if numericValue(int64(11)) != 11 {
		t.Errorf("int64 conversion failed")
	}
	if numericValue("nope") != 0 {
		t.Errorf("non-numeric should be 0")
	}
}
