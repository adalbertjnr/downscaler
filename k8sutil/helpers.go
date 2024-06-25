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
		slog.Info("downscaling message",
			"ignoring namespace", namespace,
			"reason", "already ignored from the config",
		)
		return true
	}
	return false
}

func isNamespaceAlreadyScheduled(namespace string, evicted shared.NotUsableNamespacesDuringScheduling) bool {
	if _, exists := evicted.ScheduledNamespaces[namespace]; exists {
		slog.Info("downscaling message",
			"ignoring namespace", namespace,
			"reason", "already scheduled by another routine",
		)
		return true
	}
	return false
}

func isCurrentNamespaceScheduledToDownscale(namespace string, currentNamespace string) bool {
	if strings.EqualFold(namespace, currentNamespace) {
		slog.Info("downscaling message",
			"the found namespace", namespace,
			"match the downscaler namespace", currentNamespace,
			"status", "not ignored during scheduling",
			"action", "will be last downscaled namespace",
		)
		return true
	}
	return false
}

func downscaleNamespace(ctx context.Context, k Kubernetes, namespace string) []string {
	deploymentsWithinNamespace := k.GetDeployments(ctx, namespace)

	deploymentAndReplicas := make([]string, 0)
	for _, deployment := range deploymentsWithinNamespace.Items {
		oldStateDeploymentFingerprint := fmt.Sprintf("%s,%d,%d", deployment.Name, *deployment.Spec.Replicas, shared.DeploymentsWithDownscaledState)
		deploymentAndReplicas = append(deploymentAndReplicas, oldStateDeploymentFingerprint)
		k.DownscaleDeployments(ctx, namespace, &deployment)
	}
	return deploymentAndReplicas
}

func DownscaleAnyOther(ctx context.Context, k Kubernetes, evicted shared.NotUsableNamespacesDuringScheduling) {
	if _, found := evicted.IgnoredNamespaces[shared.DownscalerNamespace]; !found {
		deployments := k.GetDeployments(ctx, shared.DownscalerNamespace)

		for _, deployment := range deployments.Items {
			k.DownscaleDeployments(ctx, shared.DownscalerNamespace, &deployment)
		}
	}
}
