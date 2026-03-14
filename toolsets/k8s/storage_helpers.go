package k8s

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/render"
)

func (t *Toolset) addCSIDriverEvidence(ctx context.Context, analysis *render.Analysis, storageClassName string, cloud cloudInfo) {
	if analysis == nil || t == nil || t.ctx.Clients == nil || t.ctx.Clients.Typed == nil {
		return
	}
	if !isAWSCloud(cloud.provider) {
		addCloudHints(analysis, cloud.provider, "storage")
		return
	}
	sc, err := t.ctx.Clients.Typed.StorageV1().StorageClasses().Get(ctx, storageClassName, metav1.GetOptions{})
	if err != nil {
		return
	}
	driverName, driverLabel := awsCSIDriverInfo(sc.Provisioner)
	if driverName == "" {
		return
	}

	drivers, err := t.ctx.Clients.Typed.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err != nil {
		analysis.AddEvidence("csiDriverError", err.Error())
		return
	}
	found := false
	for _, driver := range drivers.Items {
		if driver.Name == driverName {
			found = true
			analysis.AddEvidence("csiDriver", map[string]any{
				"name":   driver.Name,
				"attachRequired": driver.Spec.AttachRequired,
				"podInfoOnMount": driver.Spec.PodInfoOnMount,
			})
			break
		}
	}
	if !found {
		analysis.AddCause("CSI driver missing", fmt.Sprintf("CSI driver %s not found", driverName), "high")
	}

	pods, err := t.ctx.Clients.Typed.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		analysis.AddEvidence("csiPodError", err.Error())
		return
	}
	var driverPods []map[string]any
	for _, pod := range pods.Items {
		if !strings.Contains(pod.Name, driverLabel) {
			continue
		}
		driverPods = append(driverPods, map[string]any{
			"name":      pod.Name,
			"phase":     pod.Status.Phase,
			"node":      pod.Spec.NodeName,
			"ready":     isPodReady(&pod),
			"containers": len(pod.Spec.Containers),
		})
	}
	if len(driverPods) > 0 {
		analysis.AddEvidence(fmt.Sprintf("csiPods.%s", driverLabel), driverPods)
	}
}

func awsCSIDriverInfo(provisioner string) (string, string) {
	switch strings.ToLower(provisioner) {
	case "ebs.csi.aws.com":
		return "ebs.csi.aws.com", "ebs-csi"
	case "efs.csi.aws.com":
		return "efs.csi.aws.com", "efs-csi"
	default:
		return "", ""
	}
}

func summarizeCSIDriver(driver storagev1.CSIDriver) map[string]any {
	return map[string]any{
		"name":           driver.Name,
		"attachRequired": driver.Spec.AttachRequired,
		"podInfoOnMount": driver.Spec.PodInfoOnMount,
	}
}
