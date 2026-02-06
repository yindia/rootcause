package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

func (t *Toolset) handleStorageDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	pvcName := toString(req.Arguments["pvc"])
	podName := toString(req.Arguments["pod"])
	includeEvents := toBool(req.Arguments["includeEvents"], true)
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	analysis := render.NewAnalysis()
	var pvcNames []string
	if podName != "" {
		pod, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		pvcNames = collectPVCNamesFromPod(pod)
		analysis.AddEvidence("pod", map[string]any{"name": pod.Name, "pvcRefs": pvcNames})
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, pod.Name))
		if len(pvcNames) == 0 {
			analysis.AddEvidence("status", "no pvc references found in pod volumes")
			return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
		}
	} else if pvcName != "" {
		pvcNames = []string{pvcName}
	} else {
		list, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorResult(err), err
		}
		for _, pvc := range list.Items {
			pvcNames = append(pvcNames, pvc.Name)
		}
		if len(pvcNames) == 0 {
			analysis.AddEvidence("status", "no pvcs found")
			return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
		}
	}

	for _, name := range pvcNames {
		if err := t.analyzePVC(ctx, req.User, &analysis, namespace, name, includeEvents); err != nil {
			return errorResult(err), err
		}
	}

	analysis.AddNextCheck("Review storageclass provisioner and CSI driver logs")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) analyzePVC(ctx context.Context, user policy.User, analysis *render.Analysis, namespace, pvcName string, includeEvents bool) error {
	pvc, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			analysis.AddEvidence(pvcName, "pvc not found")
			analysis.AddCause("PVC missing", fmt.Sprintf("PVC %s not found", pvcName), "high")
			return nil
		}
		return err
	}
	analysis.AddResource(fmt.Sprintf("persistentvolumeclaims/%s/%s", namespace, pvc.Name))

	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}
	pvcEvidence := map[string]any{
		"phase":        pvc.Status.Phase,
		"storageClass": storageClass,
		"volumeName":   pvc.Spec.VolumeName,
		"accessModes":  pvc.Spec.AccessModes,
		"requests":     pvc.Spec.Resources.Requests,
		"conditions":   pvc.Status.Conditions,
	}
	analysis.AddEvidence(fmt.Sprintf("pvc %s", pvc.Name), pvcEvidence)

	if pvc.Status.Phase == corev1.ClaimPending {
		analysis.AddCause("PVC pending", fmt.Sprintf("PVC %s is pending", pvc.Name), "high")
	}

	if includeEvents {
		if events, err := eventsForObject(ctx, t, namespace, pvc.UID); err == nil && len(events) > 0 {
			analysis.AddEvidence(fmt.Sprintf("pvc %s events", pvc.Name), summarizeEvents(events))
		}
	}

	if storageClass != "" {
		if user.Role == policy.RoleCluster {
			sc, err := t.ctx.Clients.Typed.StorageV1().StorageClasses().Get(ctx, storageClass, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					analysis.AddCause("StorageClass missing", fmt.Sprintf("StorageClass %s not found", storageClass), "high")
				} else {
					return err
				}
			} else {
				analysis.AddEvidence(fmt.Sprintf("storageclass %s", sc.Name), map[string]any{
					"provisioner": sc.Provisioner,
					"parameters":  sc.Parameters,
					"reclaim":     sc.ReclaimPolicy,
					"volumeMode":  sc.VolumeBindingMode,
				})
				analysis.AddResource(fmt.Sprintf("storageclasses/%s", sc.Name))
			}
		} else {
			analysis.AddEvidence("storageClassCheck", "requires cluster role")
		}
	}

	if pvc.Spec.VolumeName != "" {
		if user.Role != policy.RoleCluster {
			analysis.AddEvidence("persistentVolumeCheck", "requires cluster role")
		} else {
			pv, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					analysis.AddCause("PV missing", fmt.Sprintf("PV %s not found", pvc.Spec.VolumeName), "high")
				} else {
					return err
				}
			} else {
				analysis.AddResource(fmt.Sprintf("persistentvolumes/%s", pv.Name))
				analysis.AddEvidence(fmt.Sprintf("pv %s", pv.Name), map[string]any{
					"phase":       pv.Status.Phase,
					"capacity":    pv.Spec.Capacity,
					"accessModes": pv.Spec.AccessModes,
					"storageClass": func() string {
						if pv.Spec.StorageClassName != "" {
							return pv.Spec.StorageClassName
						}
						return ""
					}(),
					"nodeAffinity": pv.Spec.NodeAffinity,
				})
				if pv.Status.Phase != corev1.VolumeBound {
					analysis.AddCause("PV not bound", fmt.Sprintf("PV %s phase %s", pv.Name, pv.Status.Phase), "medium")
				}
				if err := t.addVolumeAttachmentEvidence(ctx, analysis, pv.Name); err != nil {
					return err
				}
			}
		}
	} else if user.Role == policy.RoleCluster {
		matches, err := t.findMatchingPVs(ctx, pvc, storageClass)
		if err != nil {
			return err
		}
		if len(matches) == 0 && pvc.Status.Phase == corev1.ClaimPending {
			analysis.AddCause("No matching PV", "no available PV matches PVC requirements", "high")
		} else if len(matches) > 0 {
			analysis.AddEvidence(fmt.Sprintf("matchingPVCandidates %s", pvc.Name), matches)
		}
	}

	pods := podsUsingPVC(ctx, t, namespace, pvc.Name)
	if len(pods) > 0 {
		analysis.AddEvidence(fmt.Sprintf("podsUsingPVC %s", pvc.Name), pods)
	}
	return nil
}

func collectPVCNamesFromPod(pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var names []string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		name := volume.PersistentVolumeClaim.ClaimName
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func podsUsingPVC(ctx context.Context, t *Toolset, namespace, pvcName string) []map[string]any {
	list, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	out := []map[string]any{}
	for _, pod := range list.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil || volume.PersistentVolumeClaim.ClaimName != pvcName {
				continue
			}
			out = append(out, map[string]any{
				"name":       pod.Name,
				"phase":      pod.Status.Phase,
				"node":       pod.Spec.NodeName,
				"conditions": pod.Status.Conditions,
			})
			break
		}
	}
	return out
}

func (t *Toolset) findMatchingPVs(ctx context.Context, pvc *corev1.PersistentVolumeClaim, storageClass string) ([]string, error) {
	list, err := t.ctx.Clients.Typed.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var matches []string
	for _, pv := range list.Items {
		if pv.Status.Phase != corev1.VolumeAvailable {
			continue
		}
		if storageClass != "" && pv.Spec.StorageClassName != storageClass {
			continue
		}
		if pvc.Spec.VolumeMode != nil && pv.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode != *pv.Spec.VolumeMode {
			continue
		}
		if !accessModesMatch(pvc.Spec.AccessModes, pv.Spec.AccessModes) {
			continue
		}
		matches = append(matches, pv.Name)
	}
	sort.Strings(matches)
	return matches, nil
}

func accessModesMatch(requested, available []corev1.PersistentVolumeAccessMode) bool {
	if len(requested) == 0 {
		return true
	}
	set := map[corev1.PersistentVolumeAccessMode]struct{}{}
	for _, mode := range available {
		set[mode] = struct{}{}
	}
	for _, mode := range requested {
		if _, ok := set[mode]; !ok {
			return false
		}
	}
	return true
}

func (t *Toolset) addVolumeAttachmentEvidence(ctx context.Context, analysis *render.Analysis, pvName string) error {
	attachments, err := t.ctx.Clients.Typed.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	var related []storagev1.VolumeAttachment
	for _, attachment := range attachments.Items {
		if attachment.Spec.Source.PersistentVolumeName != nil && *attachment.Spec.Source.PersistentVolumeName == pvName {
			related = append(related, attachment)
		}
	}
	if len(related) == 0 {
		return nil
	}
	for _, attachment := range related {
		analysis.AddResource(fmt.Sprintf("volumeattachments/%s", attachment.Name))
		details := map[string]any{
			"node":      attachment.Spec.NodeName,
			"attached":  attachment.Status.Attached,
			"attachErr": attachment.Status.AttachError,
			"detachErr": attachment.Status.DetachError,
		}
		analysis.AddEvidence(fmt.Sprintf("volumeAttachment %s", attachment.Name), details)
		if attachment.Status.AttachError != nil {
			analysis.AddCause("VolumeAttachment error", fmt.Sprintf("VolumeAttachment %s failed: %s", attachment.Name, attachment.Status.AttachError.Message), "high")
		}
		if attachment.Status.DetachError != nil {
			analysis.AddCause("VolumeAttachment detach error", fmt.Sprintf("VolumeAttachment %s detach failed: %s", attachment.Name, attachment.Status.DetachError.Message), "medium")
		}
	}
	return nil
}

func eventsForObject(ctx context.Context, t *Toolset, namespace string, uid types.UID) ([]corev1.Event, error) {
	if uid == "" || namespace == "" {
		return nil, nil
	}
	list, err := t.ctx.Clients.Typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s", uid),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
