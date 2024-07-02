package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
	v1 "k8s.io/api/apps/v1"
)

func filterDeploymentsByNamespace(ctx context.Context, namespaces []string, k KubernetesImpl) map[string]*v1.Deployment {
	deploymentListMap := make(map[string]*v1.Deployment)
	for _, namespace := range namespaces {
		deploymentList := k.GetDeployments(ctx, namespace)

		for _, deployment := range deploymentList.Items {
			deploymentListMap[deployment.Name] = &deployment
		}
	}

	return deploymentListMap
}

func getMetadataReplicas(state string) (name string, replicas int32) {
	parts := strings.Split(state, ",")
	replicasInt, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.Error("get replicas conversion error", "err", err)
	}
	deploymentName := parts[0]

	return deploymentName, int32(replicasInt)
}

func generateScalePatch(updateScale int32) ([]byte, error) {
	patch := struct {
		Spec struct {
			Replicas *int32 `json:"replicas"`
		} `json:"spec"`
	}{
		Spec: struct {
			Replicas *int32 `json:"replicas"`
		}{
			Replicas: &updateScale,
		},
	}

	return json.Marshal(patch)
}

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

func downscaleNamespace(ctx context.Context, k Kubernetes, namespace, group string) (shared.Apps, error) {
	deploymentsWithinNamespace := k.GetDeployments(ctx, namespace)

	deploymentAndReplicas := make([]string, len(deploymentsWithinNamespace.Items))
	for i, deployment := range deploymentsWithinNamespace.Items {
		deploymentAndReplicas[i] = fmt.Sprintf("%s,%s,%d,%d", group, deployment.Name, *deployment.Spec.Replicas, shared.DeploymentsWithDownscaledState)

		updateScale := int32(0)
		patchBytes, err := generateScalePatch(updateScale)
		if err != nil {
			slog.Error("patch marshaling error", "err", err)
		}

		k.ScaleDeployments(ctx, namespace, &deployment, patchBytes, updateScale)
	}

	status := shared.NotEmptyNamespace
	if len(deploymentAndReplicas) == 0 {
		status = shared.EmptyNamespace
	}

	return shared.Apps{
		Status: status,
		Group:  group,
		State:  deploymentAndReplicas,
	}, nil
}

func downscaleTheDownscaler(ctx context.Context, k Kubernetes, evicted shared.NotUsableNamespacesDuringScheduling) {
	if _, found := evicted.IgnoredNamespaces[shared.DownscalerNamespace]; !found {
		deployments := k.GetDeployments(ctx, shared.DownscalerNamespace)

		scaleUpdate := int32(0)
		patchBytes, err := generateScalePatch(scaleUpdate)
		if err != nil {
			slog.Error("patch marshaling error", "err", err)
		}

		for _, deployment := range deployments.Items {
			k.ScaleDeployments(ctx, shared.DownscalerNamespace, &deployment, patchBytes, scaleUpdate)
		}
	}
}
