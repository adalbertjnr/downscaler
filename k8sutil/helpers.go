package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
)

func isNamespaceIgnored(namespace string, evicted shared.NotUsableNamespacesDuringScheduling) bool {
	if _, exists := evicted.IgnoredNamespaces[namespace]; exists {
		return true
	}
	return false
}

func isNamespaceAlreadyScheduled(namespace string, evicted shared.NotUsableNamespacesDuringScheduling) bool {
	if _, exists := evicted.ScheduledNamespaces[namespace]; exists {
		return true
	}
	return false
}

func isDownscalerNamespaceScheduledToDownscale(namespace string) bool {
	if strings.EqualFold(namespace, shared.DownscalerNamespace) {
		slog.Info("downscaling message",
			"the found namespace", namespace,
			"match the downscaler namespace", shared.DownscalerNamespace,
			"status", "not ignored during scheduling",
			"action", "will be last downscaled namespace",
		)
		return true
	}
	return false
}

func shouldSkipNamespace(namespace string, evictedNamespaces shared.NotUsableNamespacesDuringScheduling) bool {
	return isNamespaceAlreadyScheduled(namespace, evictedNamespaces) || isNamespaceIgnored(namespace, evictedNamespaces) || isDownscalerNamespaceScheduledToDownscale(namespace)
}

func downscaleNamespace(ctx context.Context, k Kubernetes, namespace, group string) ([]string, error) {
	deploymentsWithinNamespace := k.GetDeployments(ctx, namespace)
	if len(deploymentsWithinNamespace.Items) == 0 {
		return nil, fmt.Errorf("empty deployments in the current namespace %s", namespace)
	}

	deploymentAndReplicas := make([]string, len(deploymentsWithinNamespace.Items))
	for i, deployment := range deploymentsWithinNamespace.Items {
		oldStateDeploymentFingerprint := fmt.Sprintf("%s,%s,%d,%d", group, deployment.Name, *deployment.Spec.Replicas, shared.DeploymentsWithDownscaledState)
		deploymentAndReplicas[i] = oldStateDeploymentFingerprint
		k.DownscaleDeployments(ctx, namespace, &deployment)
	}
	return deploymentAndReplicas, nil
}

func downscaleTheDownscaler(ctx context.Context, k Kubernetes, evicted shared.NotUsableNamespacesDuringScheduling) {
	if _, found := evicted.IgnoredNamespaces[shared.DownscalerNamespace]; !found {
		deployments := k.GetDeployments(ctx, shared.DownscalerNamespace)

		for _, deployment := range deployments.Items {
			k.DownscaleDeployments(ctx, shared.DownscalerNamespace, &deployment)
		}
	}
}
