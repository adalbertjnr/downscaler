package kas

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

func runUpscalingByDeploymentNameStateIndex(ctx context.Context, k KubernetesImpl, namespace string, cmValue shared.Apps, deploymentMapList map[string]*v1.Deployment) map[string]shared.Apps {
	var newState []string
	for _, cmStoredState := range cmValue.State {
		cmDeploymentName, cmDeploymentReplicas := getMetadataReplicas(cmStoredState)

		patch, err := generateScalePatch(cmDeploymentReplicas)
		if err != nil {
			slog.Error("generating patch error", "err", err)
			continue
		}

		deployment := deploymentMapList[cmDeploymentName]
		k.ScaleDeployments(ctx, namespace, deployment, patch, cmDeploymentReplicas)

		stateAfterUpscaling := createNewStateIndex(cmStoredState)
		newState = append(newState, stateAfterUpscaling)
	}
	return map[string]shared.Apps{
		namespace: {
			Status: cmValue.Status,
			Group:  cmValue.Group,
			State:  newState,
		},
	}
}

func createNewStateIndex(previousState string) string {
	previousStateParts := strings.Split(previousState, ",")
	return fmt.Sprintf("%s,%s,%d", previousStateParts[0], previousStateParts[1], shared.DeploymentsWithUpscaledState)
}

func extractIndexByNamespaces(cmCurrentState map[string]shared.Apps, namespaces []string) map[string]shared.Apps {
	nsIndex := make(map[string]shared.Apps, len(namespaces))
	for _, namespace := range namespaces {
		if value, found := cmCurrentState[namespace+".yaml"]; found {
			nsIndex[namespace+".yaml"] = value
		}
	}

	return nsIndex
}

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

func downscaleNamespace(ctx context.Context, k Kubernetes, namespace, group string) (shared.Apps, error) {
	deploymentsWithinNamespace := k.GetDeployments(ctx, namespace)

	deploymentAndReplicas := make([]string, len(deploymentsWithinNamespace.Items))
	for i, deployment := range deploymentsWithinNamespace.Items {
		deploymentAndReplicas[i] = fmt.Sprintf("%s,%d,%d", deployment.Name, *deployment.Spec.Replicas, shared.DeploymentsWithDownscaledState)

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
