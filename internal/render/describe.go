package render

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"rootcause/internal/evidence"
	"rootcause/internal/redact"
)

func DescribeAnalysis(ctx context.Context, collector evidence.Collector, redactor *redact.Redactor, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) Analysis {
	analysis := NewAnalysis()
	if obj == nil {
		analysis.AddEvidence("status", "object not found")
		return analysis
	}
	analysis.AddEvidence("object", redactObject(redactor, obj))
	if collector != nil {
		events, err := collector.EventsForObject(ctx, obj)
		if err == nil && len(events) > 0 {
			analysis.AddEvidence("events", events)
		}
		owners, err := collector.OwnerChain(ctx, obj)
		if err == nil && len(owners) > 0 {
			analysis.AddEvidence("ownerChain", owners)
		}
		analysis.AddResource(collector.ResourceRef(gvr, obj.GetNamespace(), obj.GetName()))
	}
	analysis.AddNextCheck("Inspect recent changes and related controller logs")
	return analysis
}

func redactObject(redactor *redact.Redactor, obj *unstructured.Unstructured) map[string]any {
	if obj == nil {
		return map[string]any{}
	}
	data := obj.UnstructuredContent()
	if obj.GetKind() == "Secret" {
		if dataObj, ok := data["data"].(map[string]any); ok {
			for k := range dataObj {
				dataObj[k] = "[REDACTED]"
			}
		}
		if _, ok := data["stringData"]; ok {
			data["stringData"] = "[REDACTED]"
		}
	}
	if redactor == nil {
		return data
	}
	return redactor.RedactMap(data)
}
