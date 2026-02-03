package render

import "time"

type Analysis struct {
	LikelyRootCauses      []Cause        `json:"likelyRootCauses"`
	Evidence              []EvidenceItem `json:"evidence"`
	RecommendedNextChecks []string       `json:"recommendedNextChecks"`
	ResourcesExamined     []string       `json:"resourcesExamined"`
	GeneratedAt           time.Time      `json:"generatedAt"`
}

type Cause struct {
	Summary  string `json:"summary"`
	Details  string `json:"details,omitempty"`
	Severity string `json:"severity,omitempty"`
}

type EvidenceItem struct {
	Summary string `json:"summary"`
	Details any    `json:"details,omitempty"`
}

type Renderer interface {
	Render(analysis Analysis) map[string]any
}

type JSONRenderer struct{}

func NewRenderer() *JSONRenderer {
	return &JSONRenderer{}
}

func (r *JSONRenderer) Render(analysis Analysis) map[string]any {
	return map[string]any{
		"likelyRootCauses":      analysis.LikelyRootCauses,
		"evidence":              analysis.Evidence,
		"recommendedNextChecks": analysis.RecommendedNextChecks,
		"resourcesExamined":     analysis.ResourcesExamined,
		"generatedAt":           analysis.GeneratedAt,
	}
}

func NewAnalysis() Analysis {
	return Analysis{GeneratedAt: time.Now()}
}

func (a *Analysis) AddCause(summary, details, severity string) {
	a.LikelyRootCauses = append(a.LikelyRootCauses, Cause{Summary: summary, Details: details, Severity: severity})
}

func (a *Analysis) AddEvidence(summary string, details any) {
	a.Evidence = append(a.Evidence, EvidenceItem{Summary: summary, Details: details})
}

func (a *Analysis) AddNextCheck(check string) {
	a.RecommendedNextChecks = append(a.RecommendedNextChecks, check)
}

func (a *Analysis) AddResource(ref string) {
	a.ResourcesExamined = append(a.ResourcesExamined, ref)
}
