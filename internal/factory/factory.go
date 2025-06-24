package factory

import (
	"context"
	"database/sql"
	"errors"

	downscalergov1alpha1 "github.com/adalbertjnr/downscaler/api/v1alpha1"
	"github.com/adalbertjnr/downscaler/internal/client"
	"github.com/adalbertjnr/downscaler/internal/store"
	"github.com/adalbertjnr/downscaler/internal/types"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
)

type ResourceScaler interface {
	Run(downscalerObject downscalergov1alpha1.Downscaler, ruleName, namespace string, replicas types.ScalingOperation) error
}

type ScaleDeployment struct {
	Client *client.APIClient
	Logger logr.Logger

	selfNamespace map[string]downscalerDeploymentMetadata

	persistence bool
	storeClient *store.Persistence
}

var (
	ErrNotErrorDisabledPersitence = errors.New("persistence is disabled")
	ErrNotErrorOperationUpscale   = errors.New("upscale operation. no need to write the replicas in the database")
	ErrNotErrorOperationDownscale = errors.New("downscale operation. no need to read the replicas in the database")
)

type downscalerDeploymentMetadata struct {
	deployment             appsv1.Deployment
	scalingOperationObject store.ScalingOperation
}

func readReplicas(ctx context.Context, sc *store.Persistence, persistence bool, defaultScalingObject *store.ScalingOperation) error {
	if !persistence {
		return ErrNotErrorDisabledPersitence
	}

	if err := sc.ScalingOperation.Get(ctx, defaultScalingObject); err != nil {
		return err
	}

	return nil
}

func writeReplicas(ctx context.Context, sc *store.Persistence, persistence bool, currentObjectReplicas int32, defaultScalingObject *store.ScalingOperation) error {
	if !persistence {
		return ErrNotErrorDisabledPersitence
	}

	operationTypeReplicas := defaultScalingObject.Replicas
	defaultScalingObject.Replicas = int(currentObjectReplicas)

	if err := sc.ScalingOperation.Update(ctx, defaultScalingObject); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := sc.ScalingOperation.Insert(ctx, defaultScalingObject); err != nil {
				return err
			}
		}
	}

	defaultScalingObject.Replicas = operationTypeReplicas
	return nil
}

func (sc *ScaleDeployment) Run(downscalerObject downscalergov1alpha1.Downscaler, RuleNameDescription, objectNamespace string, operationTypeReplicas types.ScalingOperation) error {
	var deployments appsv1.DeploymentList
	if err := sc.Client.Get(objectNamespace, &deployments); err != nil {
		return err
	}

	defer func() {
		if object, exists := sc.selfNamespace[downscalerObject.Name]; exists {
			if err := sc.Client.Patch(object.scalingOperationObject.Replicas, &object.deployment); err != nil {
				sc.Logger.Error(err, "client", "name", downscalerObject.Name, "self patching error", err)
			}
			delete(sc.selfNamespace, object.deployment.Name)
		}
	}()

	for _, deployment := range deployments.Items {
		currentObjectReplicas := *deployment.Spec.Replicas

		defaultScalingObjectValues := store.ScalingOperation{
			ResourceName:        deployment.Name,
			RuleNameDescription: RuleNameDescription,
			NamespaceName:       objectNamespace,
			ResourceType:        types.DeploymentObjectResource.String(),
			Replicas:            int(operationTypeReplicas),
		}

		if operationTypeReplicas == types.OperationDownscale {
			if deployment.Name == downscalerObject.Name {
				sc.selfNamespace[downscalerObject.Name] = downscalerDeploymentMetadata{
					deployment:             deployment,
					scalingOperationObject: defaultScalingObjectValues,
				}
				continue
			}

			if err := writeReplicas(
				context.Background(),
				sc.storeClient,
				sc.persistence,
				currentObjectReplicas,
				&defaultScalingObjectValues,
			); err != nil {
				if !errors.Is(err, ErrNotErrorDisabledPersitence) {
					return err
				}
			}
		}

		if operationTypeReplicas == types.OperationUpscale {
			if err := readReplicas(
				context.Background(),
				sc.storeClient,
				sc.persistence,
				&defaultScalingObjectValues,
			); err != nil {
				if !errors.Is(err, ErrNotErrorDisabledPersitence) {
					sc.Logger.Error(err, "database", "reading replicas error", err)
					return err
				}
			}
		}

		if err := sc.Client.Patch(defaultScalingObjectValues.Replicas, &deployment); err != nil {
			sc.Logger.Error(err, "client", "error patching deployment", err)
			return err
		}

		sc.Logger.Info("client",
			"patching deployment", deployment.Name,
			"namespace", objectNamespace,
			"before", currentObjectReplicas,
			"after", defaultScalingObjectValues.Replicas,
			"object", defaultScalingObjectValues,
		)
	}

	return nil
}

type ScaleStatefulSet struct {
	client *client.APIClient
	logger logr.Logger

	persistence bool
	storeClient *store.Persistence
}

func (sc *ScaleStatefulSet) Run(downscalerObject downscalergov1alpha1.Downscaler, ruleNameDescription, objectNamespace string, operationTypeReplicas types.ScalingOperation) error {
	var statefulSets appsv1.StatefulSetList
	if err := sc.client.Get(objectNamespace, &statefulSets); err != nil {
		return err
	}

	for _, statefulSet := range statefulSets.Items {
		currentObjectReplicas := *statefulSet.Spec.Replicas

		defaultScalingObjectValues := store.ScalingOperation{
			RuleNameDescription: ruleNameDescription,
			ResourceName:        statefulSet.Name,
			NamespaceName:       objectNamespace,
			ResourceType:        types.StatefulSetObjectResource.String(),
			Replicas:            int(operationTypeReplicas),
		}

		if operationTypeReplicas == types.OperationDownscale {
			if err := writeReplicas(context.Background(),
				sc.storeClient,
				sc.persistence,
				currentObjectReplicas,
				&defaultScalingObjectValues,
			); err != nil {
				if !errors.Is(err, ErrNotErrorDisabledPersitence) {
					return err
				}
			}
		}

		if operationTypeReplicas == types.OperationUpscale {
			if err := readReplicas(
				context.Background(),
				sc.storeClient,
				sc.persistence,
				&defaultScalingObjectValues,
			); err != nil {
				if !errors.Is(err, ErrNotErrorDisabledPersitence) {
					sc.logger.Error(err, "database", "reading replicas error", err)
					return err
				}
			}
		}

		if err := sc.client.Patch(defaultScalingObjectValues.Replicas, &statefulSet); err != nil {
			sc.logger.Error(err, "client", "error patching deployment", err)
			return err
		}

		sc.logger.Info("client",
			"patching statefulSet", statefulSet.Name,
			"namespace", objectNamespace,
			"before", currentObjectReplicas,
			"after", defaultScalingObjectValues.Replicas,
			"object", defaultScalingObjectValues,
		)
	}

	return nil
}

type FactoryScaler map[types.ResourceType]ResourceScaler

func NewScalerFactory(client *client.APIClient, store *store.Persistence, logger logr.Logger) *FactoryScaler {
	persistence := store != nil
	return &FactoryScaler{
		types.DeploymentObjectResource: &ScaleDeployment{
			Client:        client,
			Logger:        logger,
			storeClient:   store,
			persistence:   persistence,
			selfNamespace: make(map[string]downscalerDeploymentMetadata),
		},

		types.StatefulSetObjectResource: &ScaleStatefulSet{
			client:      client,
			logger:      logger,
			storeClient: store,
			persistence: persistence,
		},
	}
}
