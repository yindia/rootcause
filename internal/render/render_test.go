package render

import "testing"

func TestRenderAnalysis(t *testing.T) {
	analysis := NewAnalysis()
	analysis.AddCause("cause", "details", "high")
	analysis.AddEvidence("evidence", map[string]any{"k": "v"})
	analysis.AddNextCheck("check logs")
	analysis.AddResource("pods/default/demo")

	renderer := NewRenderer()
	out := renderer.Render(analysis)
	if _, ok := out["likelyRootCauses"]; !ok {
		t.Fatalf("expected likelyRootCauses")
	}
	if _, ok := out["evidence"]; !ok {
		t.Fatalf("expected evidence")
	}
	if _, ok := out["recommendedNextChecks"]; !ok {
		t.Fatalf("expected recommendedNextChecks")
	}
	if _, ok := out["resourcesExamined"]; !ok {
		t.Fatalf("expected resourcesExamined")
	}
}
