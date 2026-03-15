package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	"rootcause/internal/mcp"
)

type bestPracticeCheck struct {
	ID             string         `json:"id"`
	Status         string         `json:"status"`
	Severity       string         `json:"severity"`
	Message        string         `json:"message"`
	Recommendation string         `json:"recommendation,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}

func (t *Toolset) handleBestPractice(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	kind := strings.TrimSpace(toString(args["kind"]))
	name := strings.TrimSpace(toString(args["name"]))
	namespace := strings.TrimSpace(toString(args["namespace"]))
	if kind == "" || name == "" || namespace == "" {
		err := errors.New("kind, name, and namespace are required")
		return errorResult(err), err
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	checks := make([]bestPracticeCheck, 0)
	recommendationSet := map[string]struct{}{}
	score := 100
	degrade := func(severity, status string) {
		if status == "pass" {
			return
		}
		switch severity {
		case "high":
			score -= 20
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}
	addCheck := func(item bestPracticeCheck) {
		if item.Status == "" {
			item.Status = "warn"
		}
		if item.Severity == "" {
			item.Severity = "low"
		}
		degrade(item.Severity, item.Status)
		if item.Recommendation != "" {
			recommendationSet[item.Recommendation] = struct{}{}
		}
		checks = append(checks, item)
	}

	var (
		objKind        string
		workloadName   string
		template       *corev1.PodTemplateSpec
		selector       *metav1.LabelSelector
		replicas       int32 = 1
		strategyType   string
		partition      *int32
		maxUnavailable *intstr.IntOrString
		claimTemplates []corev1.PersistentVolumeClaim
	)

	switch strings.ToLower(kind) {
	case "deployment":
		dep, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		objKind = "Deployment"
		workloadName = dep.Name
		template = &dep.Spec.Template
		selector = dep.Spec.Selector
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}
		strategyType = string(dep.Spec.Strategy.Type)
		if dep.Spec.Strategy.RollingUpdate != nil {
			maxUnavailable = dep.Spec.Strategy.RollingUpdate.MaxUnavailable
		}
		if replicas < 2 {
			addCheck(bestPracticeCheck{ID: "replicas", Status: "warn", Severity: "medium", Message: "Deployment has fewer than 2 replicas", Recommendation: "Use at least 2 replicas for HA when possible.", Details: map[string]any{"replicas": replicas}})
		} else {
			addCheck(bestPracticeCheck{ID: "replicas", Status: "pass", Severity: "low", Message: "Replica count supports basic HA", Details: map[string]any{"replicas": replicas}})
		}
	case "daemonset":
		ds, err := t.ctx.Clients.Typed.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		objKind = "DaemonSet"
		workloadName = ds.Name
		template = &ds.Spec.Template
		selector = ds.Spec.Selector
		strategyType = string(ds.Spec.UpdateStrategy.Type)
		if ds.Spec.UpdateStrategy.RollingUpdate != nil {
			maxUnavailable = ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable
		}
	case "statefulset":
		sts, err := t.ctx.Clients.Typed.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		objKind = "StatefulSet"
		workloadName = sts.Name
		template = &sts.Spec.Template
		selector = sts.Spec.Selector
		claimTemplates = append(claimTemplates, sts.Spec.VolumeClaimTemplates...)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		strategyType = string(sts.Spec.UpdateStrategy.Type)
		if sts.Spec.UpdateStrategy.RollingUpdate != nil {
			partition = sts.Spec.UpdateStrategy.RollingUpdate.Partition
		}
		if partition != nil && *partition >= replicas {
			addCheck(bestPracticeCheck{ID: "statefulset-partition", Status: "fail", Severity: "high", Message: "StatefulSet partition prevents updates for all replicas", Recommendation: "Set rollingUpdate.partition lower than replica count.", Details: map[string]any{"partition": *partition, "replicas": replicas}})
		}
		if sts.Spec.PersistentVolumeClaimRetentionPolicy != nil {
			if sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted == appsv1.DeletePersistentVolumeClaimRetentionPolicyType {
				addCheck(bestPracticeCheck{ID: "pvc-retention-when-deleted", Status: "fail", Severity: "high", Message: "PVC retention deletes data when StatefulSet is deleted", Recommendation: "Use Retain for whenDeleted unless data loss is acceptable."})
			}
			if sts.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled == appsv1.DeletePersistentVolumeClaimRetentionPolicyType {
				addCheck(bestPracticeCheck{ID: "pvc-retention-when-scaled", Status: "warn", Severity: "medium", Message: "PVC retention deletes data when StatefulSet scales down", Recommendation: "Use Retain for whenScaled to avoid accidental data loss."})
			}
		}
		if sts.Spec.PodManagementPolicy == appsv1.ParallelPodManagement {
			addCheck(bestPracticeCheck{ID: "pod-management-policy", Status: "warn", Severity: "medium", Message: "StatefulSet uses Parallel pod management", Recommendation: "Prefer OrderedReady for safer recovery in quorum-sensitive systems."})
		}
		if replicas < 2 {
			addCheck(bestPracticeCheck{ID: "replicas", Status: "warn", Severity: "medium", Message: "StatefulSet has fewer than 2 replicas", Recommendation: "Use at least 2 replicas if the workload supports redundancy.", Details: map[string]any{"replicas": replicas}})
		}
	default:
		err := fmt.Errorf("unsupported kind: %s (expected Deployment, DaemonSet, StatefulSet)", kind)
		return errorResult(err), err
	}

	normalizedStrategy := strategyType
	if normalizedStrategy == "" {
		normalizedStrategy = "RollingUpdate"
	}
	switch {
	case objKind == "Deployment" && normalizedStrategy == "Recreate":
		addCheck(bestPracticeCheck{ID: "update-strategy", Status: "fail", Severity: "high", Message: "Deployment uses Recreate update strategy", Recommendation: "Use RollingUpdate to avoid full downtime during deployment and restart.", Details: map[string]any{"strategy": normalizedStrategy}})
	case normalizedStrategy == "OnDelete" && objKind == "DaemonSet":
		addCheck(bestPracticeCheck{ID: "update-strategy", Status: "fail", Severity: "high", Message: "DaemonSet uses OnDelete update strategy", Recommendation: "Use RollingUpdate so node-level agents update safely without manual pod deletion.", Details: map[string]any{"strategy": normalizedStrategy}})
	case normalizedStrategy == "OnDelete":
		addCheck(bestPracticeCheck{ID: "update-strategy", Status: "warn", Severity: "medium", Message: "Workload uses OnDelete update strategy", Recommendation: "Use RollingUpdate for safer automated rollouts unless manual control is required.", Details: map[string]any{"strategy": normalizedStrategy}})
	case normalizedStrategy == "RollingUpdate":
		addCheck(bestPracticeCheck{ID: "update-strategy", Status: "pass", Severity: "low", Message: "Workload uses RollingUpdate strategy", Details: map[string]any{"strategy": normalizedStrategy}})
	default:
		addCheck(bestPracticeCheck{ID: "update-strategy", Status: "warn", Severity: "medium", Message: "Workload uses a non-standard update strategy", Recommendation: "Prefer RollingUpdate for resilient restart and node-recreate behavior.", Details: map[string]any{"strategy": normalizedStrategy}})
	}

	if maxUnavailable != nil {
		scaled, err := intstr.GetScaledValueFromIntOrPercent(maxUnavailable, bpMax(1, int(replicas)), false)
		if err == nil {
			ratio := float64(scaled) / float64(bpMax(1, int(replicas)))
			if ratio > 0.5 {
				addCheck(bestPracticeCheck{ID: "max-unavailable", Status: "warn", Severity: "medium", Message: "maxUnavailable is high for rollout safety", Recommendation: "Keep maxUnavailable conservative (<=50%) to reduce blast radius.", Details: map[string]any{"maxUnavailable": scaled, "ratio": ratio}})
			} else {
				addCheck(bestPracticeCheck{ID: "max-unavailable", Status: "pass", Severity: "low", Message: "maxUnavailable is within safer bounds", Details: map[string]any{"maxUnavailable": scaled, "ratio": ratio}})
			}
		}
	}

	if template == nil {
		err := errors.New("workload pod template is missing")
		return errorResult(err), err
	}
	podSpec := template.Spec
	podLabels := template.Labels
	pvcVolumeClaims := pvcClaimsFromVolumes(podSpec.Volumes)
	hasPVCReferences := len(pvcVolumeClaims) > 0 || len(claimTemplates) > 0
	pvcProfiles, profileWarnings := t.resolvePVCProfiles(ctx, namespace, workloadName, replicas, pvcVolumeClaims, claimTemplates)
	for _, warn := range profileWarnings {
		addCheck(bestPracticeCheck{ID: "pvc-profile-resolution", Status: "warn", Severity: "low", Message: warn, Recommendation: "Verify PVC/StorageClass references for this workload."})
	}

	if objKind == "DaemonSet" {
		addCheck(bestPracticeCheck{ID: "topology-spread", Status: "pass", Severity: "low", Message: "DaemonSet naturally spreads pods across nodes"})
		addCheck(bestPracticeCheck{ID: "pod-anti-affinity", Status: "pass", Severity: "low", Message: "Pod anti-affinity is optional for DaemonSet workloads"})
	} else {
		hasTSC := len(podSpec.TopologySpreadConstraints) > 0
		hasPodAntiAffinity := hasAnyPodAntiAffinityTerms(podSpec.Affinity)
		if replicas >= 2 && !hasTSC && !hasPodAntiAffinity {
			addCheck(bestPracticeCheck{ID: "topology-spread", Status: "warn", Severity: "medium", Message: "No topology spread or anti-affinity configured for multi-replica workload", Recommendation: "Add topologySpreadConstraints and/or podAntiAffinity to reduce node/zone blast radius."})
		} else if hasTSC {
			addCheck(bestPracticeCheck{ID: "topology-spread", Status: "pass", Severity: "low", Message: "Topology spread constraints are configured"})
		} else if replicas >= 2 {
			addCheck(bestPracticeCheck{ID: "topology-spread", Status: "warn", Severity: "low", Message: "Topology spread constraints are not configured", Recommendation: "Add topologySpreadConstraints for explicit node/zone placement guarantees during node recreation."})
		} else {
			addCheck(bestPracticeCheck{ID: "topology-spread", Status: "pass", Severity: "low", Message: "Topology spread is optional for single-replica workloads"})
		}
		if hasPodAntiAffinity {
			addCheck(bestPracticeCheck{ID: "pod-anti-affinity", Status: "pass", Severity: "low", Message: "Pod anti-affinity is configured"})
		} else {
			addCheck(bestPracticeCheck{ID: "pod-anti-affinity", Status: "warn", Severity: "low", Message: "Pod anti-affinity not configured", Recommendation: "Use podAntiAffinity to reduce single-node failure impact."})
		}
		if replicas >= 2 {
			hostnameSpread := hasHostnameSpreadConstraint(podSpec.TopologySpreadConstraints)
			hostnameAntiAffinity := hasHostnamePodAntiAffinity(podSpec.Affinity)
			if !hostnameSpread && !hostnameAntiAffinity {
				addCheck(bestPracticeCheck{ID: "node-spread", Status: "warn", Severity: "medium", Message: "No strong node-level spread rule detected", Recommendation: "Use hostname topology spread or required pod anti-affinity so replicas avoid co-locating on one node."})
			} else {
				addCheck(bestPracticeCheck{ID: "node-spread", Status: "pass", Severity: "low", Message: "Node-level spread rule is configured"})
			}
		}
	}

	if podSpec.TerminationGracePeriodSeconds == nil || *podSpec.TerminationGracePeriodSeconds <= 0 {
		addCheck(bestPracticeCheck{ID: "termination-grace", Status: "warn", Severity: "low", Message: "terminationGracePeriodSeconds is missing or zero", Recommendation: "Set terminationGracePeriodSeconds to allow graceful shutdown."})
	} else {
		addCheck(bestPracticeCheck{ID: "termination-grace", Status: "pass", Severity: "low", Message: "terminationGracePeriodSeconds is configured", Details: map[string]any{"seconds": *podSpec.TerminationGracePeriodSeconds}})
	}

	if podSpec.SecurityContext == nil || podSpec.SecurityContext.SeccompProfile == nil {
		addCheck(bestPracticeCheck{ID: "pod-seccomp", Status: "fail", Severity: "high", Message: "Pod seccomp profile is not configured", Recommendation: "Set pod securityContext.seccompProfile to RuntimeDefault."})
	} else {
		addCheck(bestPracticeCheck{ID: "pod-seccomp", Status: "pass", Severity: "low", Message: "Pod seccomp profile is configured"})
	}
	if podSpec.SecurityContext == nil || podSpec.SecurityContext.RunAsNonRoot == nil || !*podSpec.SecurityContext.RunAsNonRoot {
		addCheck(bestPracticeCheck{ID: "run-as-non-root", Status: "warn", Severity: "medium", Message: "runAsNonRoot is not enforced at pod level", Recommendation: "Set pod securityContext.runAsNonRoot=true."})
	} else {
		addCheck(bestPracticeCheck{ID: "run-as-non-root", Status: "pass", Severity: "low", Message: "runAsNonRoot is enforced"})
	}

	containers := append([]corev1.Container{}, podSpec.Containers...)
	containers = append(containers, podSpec.InitContainers...)
	for _, c := range containers {
		checkIDPrefix := "container:" + c.Name + ":"
		if c.ReadinessProbe == nil {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "readiness-probe", Status: "warn", Severity: "medium", Message: "Container missing readiness probe", Recommendation: "Add readinessProbe for safer rollouts.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "readiness-probe", Status: "pass", Severity: "low", Message: "Readiness probe configured", Details: map[string]any{"container": c.Name}})
		}
		if c.LivenessProbe == nil {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "liveness-probe", Status: "warn", Severity: "medium", Message: "Container missing liveness probe", Recommendation: "Add livenessProbe for automatic recovery.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "liveness-probe", Status: "pass", Severity: "low", Message: "Liveness probe configured", Details: map[string]any{"container": c.Name}})
		}
		if c.Resources.Requests.Cpu().IsZero() || c.Resources.Requests.Memory().IsZero() {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "resource-requests", Status: "warn", Severity: "medium", Message: "CPU/memory requests are incomplete", Recommendation: "Set CPU and memory requests for reliable scheduling.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "resource-requests", Status: "pass", Severity: "low", Message: "Resource requests are configured", Details: map[string]any{"container": c.Name}})
		}
		if c.Resources.Limits.Cpu().IsZero() || c.Resources.Limits.Memory().IsZero() {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "resource-limits", Status: "warn", Severity: "low", Message: "CPU/memory limits are incomplete", Recommendation: "Set resource limits to prevent noisy-neighbor issues.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "resource-limits", Status: "pass", Severity: "low", Message: "Resource limits are configured", Details: map[string]any{"container": c.Name}})
		}
		if strings.HasSuffix(c.Image, ":latest") || (!strings.Contains(c.Image, ":") && !strings.Contains(c.Image, "@")) {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "image-tag", Status: "warn", Severity: "medium", Message: "Container image uses floating tag", Recommendation: "Pin immutable image tags or digests for reproducible recovery.", Details: map[string]any{"container": c.Name, "image": c.Image}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "image-tag", Status: "pass", Severity: "low", Message: "Container image appears pinned", Details: map[string]any{"container": c.Name, "image": c.Image}})
		}
		if c.SecurityContext == nil || c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "allow-priv-escalation", Status: "fail", Severity: "high", Message: "allowPrivilegeEscalation is not disabled", Recommendation: "Set container securityContext.allowPrivilegeEscalation=false.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "allow-priv-escalation", Status: "pass", Severity: "low", Message: "allowPrivilegeEscalation is disabled", Details: map[string]any{"container": c.Name}})
		}
		if c.SecurityContext == nil || c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "read-only-rootfs", Status: "warn", Severity: "low", Message: "readOnlyRootFilesystem is not enabled", Recommendation: "Enable readOnlyRootFilesystem when feasible.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "read-only-rootfs", Status: "pass", Severity: "low", Message: "readOnlyRootFilesystem is enabled", Details: map[string]any{"container": c.Name}})
		}
		if c.SecurityContext == nil || c.SecurityContext.Capabilities == nil || len(c.SecurityContext.Capabilities.Drop) == 0 {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "capabilities-drop", Status: "warn", Severity: "medium", Message: "Container does not drop Linux capabilities", Recommendation: "Drop ALL capabilities unless explicitly required.", Details: map[string]any{"container": c.Name}})
		} else {
			addCheck(bestPracticeCheck{ID: checkIDPrefix + "capabilities-drop", Status: "pass", Severity: "low", Message: "Container drops capabilities", Details: map[string]any{"container": c.Name}})
		}
	}

	if hasPVCReferences {
		if podSpec.TerminationGracePeriodSeconds == nil || *podSpec.TerminationGracePeriodSeconds < 10 {
			addCheck(bestPracticeCheck{ID: "pvc-termination-grace", Status: "fail", Severity: "high", Message: "terminationGracePeriodSeconds is too low for PVC-backed workloads", Recommendation: "Set terminationGracePeriodSeconds >= 30 to reduce detach/attach delays during restart."})
		} else if *podSpec.TerminationGracePeriodSeconds < 30 {
			addCheck(bestPracticeCheck{ID: "pvc-termination-grace", Status: "warn", Severity: "medium", Message: "terminationGracePeriodSeconds may be short for PVC detach/attach cycles", Recommendation: "Set terminationGracePeriodSeconds >= 30 for stateful/PVC workloads."})
		} else {
			addCheck(bestPracticeCheck{ID: "pvc-termination-grace", Status: "pass", Severity: "low", Message: "terminationGracePeriodSeconds is reasonable for PVC-backed restart"})
		}

		for _, c := range podSpec.Containers {
			if containerMountsPVC(c, pvcVolumeClaims) && (c.Lifecycle == nil || c.Lifecycle.PreStop == nil) {
				addCheck(bestPracticeCheck{ID: "container:" + c.Name + ":pvc-prestop", Status: "warn", Severity: "medium", Message: "PVC-mounted container has no preStop hook", Recommendation: "Add a preStop hook to give workloads time to flush I/O before volume detach.", Details: map[string]any{"container": c.Name}})
			} else if containerMountsPVC(c, pvcVolumeClaims) {
				addCheck(bestPracticeCheck{ID: "container:" + c.Name + ":pvc-prestop", Status: "pass", Severity: "low", Message: "PVC-mounted container has a preStop hook", Details: map[string]any{"container": c.Name}})
			}
		}

		if isMultiPodPVCWorkload(objKind, replicas) && len(pvcProfiles) > 0 {
			if riskyClaims := sharedRWOClaims(pvcProfiles); len(riskyClaims) > 0 {
				addCheck(bestPracticeCheck{ID: "pvc-shared-rwo", Status: "fail", Severity: "high", Message: "Multi-pod workload references ReadWriteOnce PVC(s)", Recommendation: "Avoid sharing a single RWO PVC across multiple pods; use StatefulSet volumeClaimTemplates or RWX storage where appropriate.", Details: map[string]any{"claims": riskyClaims}})
			} else {
				addCheck(bestPracticeCheck{ID: "pvc-shared-rwo", Status: "pass", Severity: "low", Message: "No risky shared RWO PVC detected for this workload"})
			}
		} else if isMultiPodPVCWorkload(objKind, replicas) {
			addCheck(bestPracticeCheck{ID: "pvc-shared-rwo", Status: "warn", Severity: "medium", Message: "Unable to determine PVC access mode risk for multi-pod workload", Recommendation: "Ensure shared PVCs are not ReadWriteOnce when multiple pods can run concurrently."})
		}

		bindingChecks, bindingErr := t.storageBindingChecks(ctx, podSpec, pvcProfiles)
		if bindingErr != nil {
			addCheck(bestPracticeCheck{ID: "pvc-storageclass-binding", Status: "warn", Severity: "low", Message: "Unable to evaluate StorageClass volumeBindingMode", Recommendation: "Validate StorageClass volumeBindingMode and topology compatibility manually.", Details: map[string]any{"error": bindingErr.Error()}})
		} else {
			if len(bindingChecks) == 0 {
				addCheck(bestPracticeCheck{ID: "pvc-storageclass-binding", Status: "warn", Severity: "medium", Message: "StorageClass binding mode could not be resolved for PVC-backed workload", Recommendation: "Set storageClassName explicitly or ensure a default StorageClass is configured with WaitForFirstConsumer."})
			}
			for _, item := range bindingChecks {
				addCheck(item)
			}
		}

		attachmentChecks, attachmentErr := t.pvcAttachmentChecks(ctx, pvcProfiles)
		if attachmentErr != nil {
			addCheck(bestPracticeCheck{ID: "pvc-volume-attachment", Status: "warn", Severity: "low", Message: "Unable to evaluate VolumeAttachment state", Recommendation: "Review VolumeAttachment errors for attach/detach failures manually.", Details: map[string]any{"error": attachmentErr.Error()}})
		} else {
			for _, item := range attachmentChecks {
				addCheck(item)
			}
		}
	}

	pdbMatched, pdbNames, pdbErr := t.hasMatchingPDB(ctx, namespace, selector, podLabels)
	if pdbErr != nil {
		addCheck(bestPracticeCheck{ID: "pdb", Status: "warn", Severity: "low", Message: "Unable to evaluate PDB coverage", Recommendation: "Verify PodDisruptionBudget coverage manually.", Details: map[string]any{"error": pdbErr.Error()}})
	} else if objKind == "DaemonSet" {
		addCheck(bestPracticeCheck{ID: "pdb", Status: "pass", Severity: "low", Message: "PDB is optional for DaemonSet workloads", Details: map[string]any{"matched": pdbMatched, "pdbs": pdbNames}})
	} else if replicas > 1 && !pdbMatched {
		addCheck(bestPracticeCheck{ID: "pdb", Status: "warn", Severity: "medium", Message: "No matching PDB found for multi-replica workload", Recommendation: "Create a PodDisruptionBudget to preserve availability during disruptions.", Details: map[string]any{"replicas": replicas}})
	} else {
		addCheck(bestPracticeCheck{ID: "pdb", Status: "pass", Severity: "low", Message: "PDB coverage is present or not required", Details: map[string]any{"matched": pdbMatched, "pdbs": pdbNames}})
	}

	if score < 0 {
		score = 0
	}
	high := 0
	medium := 0
	low := 0
	for _, c := range checks {
		if c.Status == "pass" {
			continue
		}
		switch c.Severity {
		case "high":
			high++
		case "medium":
			medium++
		default:
			low++
		}
	}
	recommendations := make([]string, 0, len(recommendationSet))
	for rec := range recommendationSet {
		recommendations = append(recommendations, rec)
	}
	sort.Strings(recommendations)

	result := map[string]any{
		"workload": map[string]any{
			"kind":      objKind,
			"name":      name,
			"namespace": namespace,
		},
		"compliant":       high == 0 && medium == 0,
		"score":           score,
		"checks":          checks,
		"recommendations": recommendations,
		"summary": map[string]any{
			"high":   high,
			"medium": medium,
			"low":    low,
			"total":  len(checks),
		},
	}
	return mcp.ToolResult{Data: result, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: []string{fmt.Sprintf("%s/%s/%s", objKind, namespace, name)}}}, nil
}

func (t *Toolset) hasMatchingPDB(ctx context.Context, namespace string, workloadSelector *metav1.LabelSelector, podLabels map[string]string) (bool, []string, error) {
	if workloadSelector == nil && len(podLabels) == 0 {
		return false, nil, nil
	}
	pdbs, err := t.ctx.Clients.Typed.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, nil, err
	}
	matches := make([]string, 0)
	for _, pdb := range pdbs.Items {
		if pdb.Spec.Selector == nil {
			continue
		}
		sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}
		if len(podLabels) > 0 {
			if sel.Matches(labels.Set(podLabels)) {
				matches = append(matches, pdb.Name)
			}
			continue
		}
		if workloadSelector != nil {
			wlSel, err := metav1.LabelSelectorAsSelector(workloadSelector)
			if err == nil && selectorIntersects(sel, wlSel) {
				matches = append(matches, pdb.Name)
			}
		}
	}
	sort.Strings(matches)
	return len(matches) > 0, matches, nil
}

func selectorIntersects(a, b labels.Selector) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Empty() || b.Empty() {
		return true
	}
	for _, candidate := range []labels.Set{{"app": "placeholder"}, {"k8s-app": "placeholder"}} {
		if a.Matches(candidate) && b.Matches(candidate) {
			return true
		}
	}
	return false
}

func bpMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type pvcProfile struct {
	ClaimName        string
	StorageClass     string
	AccessModes      []corev1.PersistentVolumeAccessMode
	PersistentVolume string
	FromTemplate     bool
}

func pvcClaimsFromVolumes(volumes []corev1.Volume) map[string]string {
	out := map[string]string{}
	for _, volume := range volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		claim := strings.TrimSpace(volume.PersistentVolumeClaim.ClaimName)
		if claim == "" {
			continue
		}
		out[volume.Name] = claim
	}
	return out
}

func containerMountsPVC(container corev1.Container, claimsByVolume map[string]string) bool {
	if len(claimsByVolume) == 0 {
		return false
	}
	for _, mount := range container.VolumeMounts {
		if _, ok := claimsByVolume[mount.Name]; ok {
			return true
		}
	}
	return false
}

func hasHostnameSpreadConstraint(constraints []corev1.TopologySpreadConstraint) bool {
	for _, c := range constraints {
		if c.TopologyKey == corev1.LabelHostname && c.WhenUnsatisfiable == corev1.DoNotSchedule {
			return true
		}
	}
	return false
}

func hasHostnamePodAntiAffinity(affinity *corev1.Affinity) bool {
	if affinity == nil || affinity.PodAntiAffinity == nil {
		return false
	}
	for _, term := range affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
		if term.TopologyKey == corev1.LabelHostname {
			return true
		}
	}
	return false
}

func hasAnyPodAntiAffinityTerms(affinity *corev1.Affinity) bool {
	if affinity == nil || affinity.PodAntiAffinity == nil {
		return false
	}
	return len(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0 || len(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0
}

func isMultiPodPVCWorkload(kind string, replicas int32) bool {
	return kind == "DaemonSet" || replicas > 1
}

func sharedRWOClaims(profiles []pvcProfile) []string {
	out := make([]string, 0)
	for _, profile := range profiles {
		if profile.FromTemplate {
			continue
		}
		for _, mode := range profile.AccessModes {
			if mode == corev1.ReadWriteOnce || mode == corev1.ReadWriteOncePod {
				out = append(out, profile.ClaimName)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (t *Toolset) resolvePVCProfiles(ctx context.Context, namespace, workloadName string, replicas int32, directClaims map[string]string, claimTemplates []corev1.PersistentVolumeClaim) ([]pvcProfile, []string) {
	profiles := make([]pvcProfile, 0)
	warnings := make([]string, 0)
	seen := map[string]struct{}{}

	for _, claimName := range directClaims {
		if _, ok := seen[claimName]; ok {
			continue
		}
		seen[claimName] = struct{}{}
		pvc, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("PVC %s could not be read: %v", claimName, err))
			continue
		}
		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = strings.TrimSpace(*pvc.Spec.StorageClassName)
		}
		profiles = append(profiles, pvcProfile{
			ClaimName:        claimName,
			StorageClass:     storageClass,
			AccessModes:      pvc.Spec.AccessModes,
			PersistentVolume: pvc.Spec.VolumeName,
		})
	}

	for _, tpl := range claimTemplates {
		claimName := strings.TrimSpace(tpl.Name)
		if claimName == "" {
			continue
		}
		if _, ok := seen[claimName]; ok {
			continue
		}
		seen[claimName] = struct{}{}
		storageClass := ""
		if tpl.Spec.StorageClassName != nil {
			storageClass = strings.TrimSpace(*tpl.Spec.StorageClassName)
		}
		profiles = append(profiles, pvcProfile{
			ClaimName:    claimName,
			StorageClass: storageClass,
			AccessModes:  tpl.Spec.AccessModes,
			FromTemplate: true,
		})

		if workloadName != "" && replicas > 0 {
			firstPVCName := fmt.Sprintf("%s-%s-0", claimName, workloadName)
			if pvc, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, firstPVCName, metav1.GetOptions{}); err == nil {
				if pvc.Spec.StorageClassName != nil {
					profiles[len(profiles)-1].StorageClass = strings.TrimSpace(*pvc.Spec.StorageClassName)
				}
				profiles[len(profiles)-1].PersistentVolume = pvc.Spec.VolumeName
			}
		}
	}

	return profiles, warnings
}

func (t *Toolset) storageBindingChecks(ctx context.Context, podSpec corev1.PodSpec, profiles []pvcProfile) ([]bestPracticeCheck, error) {
	classNames := map[string]struct{}{}
	emptyStorageClassRef := false
	for _, profile := range profiles {
		if profile.StorageClass != "" {
			classNames[profile.StorageClass] = struct{}{}
		} else {
			emptyStorageClassRef = true
		}
	}
	if emptyStorageClassRef {
		defaultClass, err := t.defaultStorageClassName(ctx)
		if err != nil {
			return nil, err
		}
		if defaultClass != "" {
			classNames[defaultClass] = struct{}{}
		}
	}
	if len(classNames) == 0 {
		return nil, nil
	}

	hasSchedulingConstraints := len(podSpec.NodeSelector) > 0 || len(podSpec.TopologySpreadConstraints) > 0
	if podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil {
		hasSchedulingConstraints = true
	}

	checks := make([]bestPracticeCheck, 0)
	for className := range classNames {
		sc, err := t.ctx.Clients.Typed.StorageV1().StorageClasses().Get(ctx, className, metav1.GetOptions{})
		if err != nil {
			checks = append(checks, bestPracticeCheck{ID: "storageclass:" + className + ":binding-mode", Status: "warn", Severity: "low", Message: "Unable to read StorageClass for PVC", Recommendation: "Verify StorageClass volumeBindingMode manually.", Details: map[string]any{"storageClass": className, "error": err.Error()}})
			continue
		}
		mode := storagev1.VolumeBindingImmediate
		if sc.VolumeBindingMode != nil {
			mode = *sc.VolumeBindingMode
		}
		if mode != storagev1.VolumeBindingWaitForFirstConsumer {
			severity := "medium"
			status := "warn"
			message := "StorageClass does not use WaitForFirstConsumer"
			if hasSchedulingConstraints {
				severity = "high"
				status = "fail"
				message = "StorageClass Immediate binding can conflict with scheduling constraints"
			}
			checks = append(checks, bestPracticeCheck{ID: "storageclass:" + className + ":binding-mode", Status: status, Severity: severity, Message: message, Recommendation: "Use volumeBindingMode: WaitForFirstConsumer for zonal/block storage to reduce PVC attach/scheduling conflicts.", Details: map[string]any{"storageClass": className, "volumeBindingMode": string(mode)}})
		} else {
			checks = append(checks, bestPracticeCheck{ID: "storageclass:" + className + ":binding-mode", Status: "pass", Severity: "low", Message: "StorageClass uses WaitForFirstConsumer", Details: map[string]any{"storageClass": className, "volumeBindingMode": string(mode)}})
		}
	}

	return checks, nil
}

func (t *Toolset) pvcAttachmentChecks(ctx context.Context, profiles []pvcProfile) ([]bestPracticeCheck, error) {
	volumeNames := map[string]string{}
	for _, profile := range profiles {
		if profile.PersistentVolume != "" {
			volumeNames[profile.PersistentVolume] = profile.ClaimName
		}
	}
	if len(volumeNames) == 0 {
		return nil, nil
	}

	attachments, err := t.ctx.Clients.Typed.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	byVolume := map[string][]storagev1.VolumeAttachment{}
	for _, attachment := range attachments.Items {
		if attachment.Spec.Source.PersistentVolumeName == nil {
			continue
		}
		pv := *attachment.Spec.Source.PersistentVolumeName
		if _, ok := volumeNames[pv]; !ok {
			continue
		}
		byVolume[pv] = append(byVolume[pv], attachment)
	}

	checks := make([]bestPracticeCheck, 0)
	for pvName, claimName := range volumeNames {
		related := byVolume[pvName]
		nodes := map[string]struct{}{}
		hasAttachError := false
		hasDetachError := false
		for _, va := range related {
			nodes[va.Spec.NodeName] = struct{}{}
			if va.Status.AttachError != nil {
				hasAttachError = true
			}
			if va.Status.DetachError != nil {
				hasDetachError = true
			}
		}
		if hasAttachError {
			checks = append(checks, bestPracticeCheck{ID: "pvc-attach-error:" + claimName, Status: "fail", Severity: "high", Message: "VolumeAttachment attach errors detected", Recommendation: "Investigate CSI driver and node events; attach errors can block pod startup.", Details: map[string]any{"claim": claimName, "persistentVolume": pvName}})
		}
		if hasDetachError {
			checks = append(checks, bestPracticeCheck{ID: "pvc-detach-error:" + claimName, Status: "fail", Severity: "high", Message: "VolumeAttachment detach errors detected", Recommendation: "Investigate node/CSI detach failures; stale attachments delay rescheduling to other nodes.", Details: map[string]any{"claim": claimName, "persistentVolume": pvName}})
		}
		if len(nodes) > 1 {
			checks = append(checks, bestPracticeCheck{ID: "pvc-attach-churn:" + claimName, Status: "warn", Severity: "medium", Message: "PVC has attachments across multiple nodes", Recommendation: "Expect attach/detach startup delays after rescheduling; tune graceful termination and check CSI performance.", Details: map[string]any{"claim": claimName, "persistentVolume": pvName, "nodesSeen": len(nodes)}})
		} else {
			checks = append(checks, bestPracticeCheck{ID: "pvc-attach-churn:" + claimName, Status: "pass", Severity: "low", Message: "No multi-node attachment churn detected for PVC", Details: map[string]any{"claim": claimName, "persistentVolume": pvName, "nodesSeen": len(nodes)}})
		}
	}
	return checks, nil
}

func (t *Toolset) defaultStorageClassName(ctx context.Context) (string, error) {
	list, err := t.ctx.Clients.Typed.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, sc := range list.Items {
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" || sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true" {
			return sc.Name, nil
		}
	}
	return "", nil
}
