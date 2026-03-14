package k8s

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

type imagePullIssue struct {
	Container string `json:"container"`
	Image     string `json:"image"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
}

type imageRef struct {
	Full       string
	Registry   string
	Repository string
	Tag        string
	Digest     string
}

func hasImagePullIssue(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, status := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if status.State.Waiting == nil {
			continue
		}
		if status.State.Waiting.Reason == "ImagePullBackOff" || status.State.Waiting.Reason == "ErrImagePull" {
			return true
		}
	}
	return false
}

func collectImagePullIssues(pod *corev1.Pod) []imagePullIssue {
	if pod == nil {
		return nil
	}
	var issues []imagePullIssue
	for _, status := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if status.State.Waiting == nil {
			continue
		}
		if status.State.Waiting.Reason != "ImagePullBackOff" && status.State.Waiting.Reason != "ErrImagePull" {
			continue
		}
		issues = append(issues, imagePullIssue{
			Container: status.Name,
			Image:     status.Image,
			Reason:    status.State.Waiting.Reason,
			Message:   status.State.Waiting.Message,
		})
	}
	return issues
}

func (t *Toolset) addImagePullEvidence(ctx context.Context, req mcp.ToolRequest, analysis *render.Analysis, pod *corev1.Pod, cloud cloudInfo) {
	if analysis == nil || pod == nil {
		return
	}
	issues := collectImagePullIssues(pod)
	if len(issues) == 0 {
		return
	}
	analysis.AddCause("Image pull error", fmt.Sprintf("Pod %s has image pull failures", pod.Name), "high")
	analysis.AddEvidence(fmt.Sprintf("imagePull.%s", pod.Name), issues)

	saName := pod.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}
	if t.ctx.Clients != nil && t.ctx.Clients.Typed != nil {
		if sa, err := t.ctx.Clients.Typed.CoreV1().ServiceAccounts(pod.Namespace).Get(ctx, saName, metav1.GetOptions{}); err == nil {
			analysis.AddEvidence(fmt.Sprintf("imagePullSecrets.%s", pod.Name), map[string]any{
				"pod":                pod.Spec.ImagePullSecrets,
				"serviceAccount":     sa.ImagePullSecrets,
				"serviceAccountName": sa.Name,
			})
		}
	}

	refs := uniqueImageRefs(pod)
	if len(refs) == 0 {
		return
	}
	analysis.AddEvidence(fmt.Sprintf("imageRefs.%s", pod.Name), mapImageRefs(refs))

	if !isAWSCloud(cloud.provider) {
		addCloudHints(analysis, cloud.provider, "image")
		return
	}

	if t.ctx.Registry == nil {
		analysis.AddEvidence("awsEcr", "tool registry unavailable")
		return
	}
	if _, ok := t.ctx.Registry.Get("aws.ecr.describe_repository"); !ok {
		analysis.AddEvidence("awsEcr", "aws toolset not enabled")
		return
	}

	for _, ref := range refs {
		if !isECRImage(ref.Registry) {
			continue
		}
		repo := ref.Repository
		if repo == "" {
			continue
		}
		region := regionFromECRRegistry(ref.Registry)
		repoResult, err := t.ctx.CallTool(ctx, req.User, "aws.ecr.describe_repository", map[string]any{
			"repositoryName": repo,
			"region":         region,
		})
		if err != nil {
			analysis.AddCause("ECR repository lookup failed", err.Error(), "high")
			analysis.AddEvidence(fmt.Sprintf("ecrRepo.%s", repo), err.Error())
			continue
		}
		analysis.AddEvidence(fmt.Sprintf("ecrRepo.%s", repo), repoResult.Data)

		imageArgs := map[string]any{
			"repositoryName": repo,
			"region":         region,
		}
		if ref.Tag != "" {
			imageArgs["imageTags"] = []string{ref.Tag}
		}
		if ref.Digest != "" {
			imageArgs["imageDigests"] = []string{ref.Digest}
		}
		imageResult, err := t.ctx.CallTool(ctx, req.User, "aws.ecr.describe_images", imageArgs)
		if err != nil {
			analysis.AddCause("ECR image lookup failed", err.Error(), "high")
			analysis.AddEvidence(fmt.Sprintf("ecrImage.%s", repo), err.Error())
			continue
		}
		analysis.AddEvidence(fmt.Sprintf("ecrImage.%s", repo), imageResult.Data)
	}

	analysis.AddNextCheck("Verify node IAM role has ecr:GetAuthorizationToken and ecr:BatchGetImage")
	analysis.AddNextCheck("If using IRSA for image pulls, ensure proper imagePullSecrets or credential helper")
}

func uniqueImageRefs(pod *corev1.Pod) []imageRef {
	if pod == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var refs []imageRef
	add := func(image string) {
		image = strings.TrimSpace(image)
		if image == "" {
			return
		}
		if _, ok := seen[image]; ok {
			return
		}
		seen[image] = struct{}{}
		ref := parseImageRef(image)
		refs = append(refs, ref)
	}
	for _, container := range pod.Spec.InitContainers {
		add(container.Image)
	}
	for _, container := range pod.Spec.Containers {
		add(container.Image)
	}
	return refs
}

func mapImageRefs(refs []imageRef) []map[string]any {
	if len(refs) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		out = append(out, map[string]any{
			"image":      ref.Full,
			"registry":   ref.Registry,
			"repository": ref.Repository,
			"tag":        ref.Tag,
			"digest":     ref.Digest,
		})
	}
	return out
}

func parseImageRef(image string) imageRef {
	ref := imageRef{Full: image}
	if image == "" {
		return ref
	}
	name := image
	if strings.Contains(name, "@") {
		parts := strings.SplitN(name, "@", 2)
		name = parts[0]
		ref.Digest = parts[1]
	}
	tag := ""
	lastSlash := strings.LastIndex(name, "/")
	lastColon := strings.LastIndex(name, ":")
	if lastColon > lastSlash {
		tag = name[lastColon+1:]
		name = name[:lastColon]
	}
	ref.Tag = tag

	registry := ""
	repo := name
	if parts := strings.Split(name, "/"); len(parts) > 1 {
		candidate := parts[0]
		if strings.Contains(candidate, ".") || strings.Contains(candidate, ":") || candidate == "localhost" {
			registry = candidate
			repo = strings.Join(parts[1:], "/")
		}
	}
	if registry == "" {
		registry = "docker.io"
	}
	ref.Registry = registry
	ref.Repository = repo
	return ref
}

func isECRImage(registry string) bool {
	registry = strings.ToLower(strings.TrimSpace(registry))
	return strings.Contains(registry, "ecr.") && strings.Contains(registry, "amazonaws.com")
}

func regionFromECRRegistry(registry string) string {
	registry = strings.ToLower(strings.TrimSpace(registry))
	parts := strings.Split(registry, ".")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "ecr" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

var regionRegexp = regexp.MustCompile(`[a-z]{2}-[a-z]+-\d`)

func regionFromHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if matches := regionRegexp.FindAllString(host, -1); len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func hostFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		if parsed.Host != "" {
			return parsed.Host
		}
	}
	return strings.TrimPrefix(raw, "https://")
}
