package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	downscalergov1alpha1 "github.com/adalbertjnr/downscaler/api/v1alpha1"
	apiclient "github.com/adalbertjnr/downscaler/internal/client"
	"github.com/adalbertjnr/downscaler/internal/factory"
	"github.com/adalbertjnr/downscaler/internal/store"
	objecttypes "github.com/adalbertjnr/downscaler/internal/types"
	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	defaultFormatTime = "15:04:05"
	oneSecond         = time.Second + 50*time.Millisecond
)

type testCase struct {
	name                       string
	test                       string
	objectName                 []string
	namespaces                 []downscalergov1alpha1.Namespace
	overrideReplicas           []objecttypes.ResourceType
	expectedDownscaledReplicas int32
	initiaReplicas             int32
	expectedReplicas           int32
}

func intializeManager(t *testing.T, c *apiclient.APIClient, downscalerObject downscalergov1alpha1.Downscaler, storeClient *store.Persistence) *Downscaler {
	location, err := time.LoadLocation(downscalerObject.Spec.Schedule.TimeZone)
	if err != nil {
		t.Fatalf("error loading timezone location: %v", err)
	}

	dm := setupDownscalerInstance(c, downscalerObject, storeClient)
	dm.cron = cron.New(cron.WithLocation(location), cron.WithSeconds())

	dm.handleDatabase()
	dm.initializeCronTasks()
	return dm
}

func createObjects(objectType any, namespaces []downscalergov1alpha1.Namespace, objectNames []string, replicas int32) []client.Object {
	var clientObjectList []client.Object

	switch objectType.(type) {
	case *appsv1.Deployment:
		for i := range objectNames {
			clientObject := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      objectNames[i],
					Namespace: namespaces[i].String(),
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
			}
			clientObjectList = append(clientObjectList, clientObject)
		}
	case *appsv1.StatefulSet:
		for i := range objectNames {
			clientObject := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      objectNames[i],
					Namespace: namespaces[i].String(),
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: &replicas,
				},
			}
			clientObjectList = append(clientObjectList, clientObject)
		}
	}

	return clientObjectList
}

func createTestScaleTime(downscaleTime, upscaleTime time.Duration) (string, string) {
	now := time.Now()
	if upscaleTime == -1 {
		return now.Add(downscaleTime).Format(defaultFormatTime), ""
	}
	if downscaleTime == -1 {
		return "", now.Add(upscaleTime).Format(defaultFormatTime)
	}

	upscale := now.Add(upscaleTime).Format(defaultFormatTime)
	downscale := now.Add(downscaleTime).Format(defaultFormatTime)
	return downscale, upscale
}

func setupDownscalerInstance(c *apiclient.APIClient, downscalerObject downscalergov1alpha1.Downscaler, persistence *store.Persistence) *Downscaler {
	return (&Downscaler{}).
		Client(c).
		Factory(factory.NewScalerFactory(c, persistence, logr.Logger{})).
		Persistence(persistence).
		Add(context.Background(), downscalerObject).
		Logger(logr.Logger{})
}

func setupDownscalerObject(downscaleTime, upscaleTime string, ruleNameDescription string, namespaces []downscalergov1alpha1.Namespace, resourceTypes []objecttypes.ResourceType) downscalergov1alpha1.Downscaler {
	return downscalergov1alpha1.Downscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "downscaler.go/v1alpha1",
			Kind:       "Downscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "downscaler-test",
			Namespace: "downscaler-ns-test",
		},
		Spec: downscalergov1alpha1.DownscalerSpec{
			Config:   downscalergov1alpha1.Config{CronLoggerInterval: 60},
			Schedule: downscalergov1alpha1.Schedule{TimeZone: "America/Sao_Paulo", Recurrence: "@daily"},
			DownscalerOptions: downscalergov1alpha1.DownscalerOptions{
				ResourceScaling: []objecttypes.ResourceType{"deployments", "statefulset"},
				TimeRules: &downscalergov1alpha1.TimeRules{
					Rules: []downscalergov1alpha1.Rules{
						{
							Name:            ruleNameDescription,
							Namespaces:      namespaces,
							DownscaleTime:   downscaleTime,
							UpscaleTime:     upscaleTime,
							OverrideScaling: resourceTypes,
						},
					},
				},
			},
		},
	}
}

func TestDownscalingDeployments(t *testing.T) {

	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:             "TestSingleNamespaceSingleDeployment",
			name:             "single namespace. downscale deployment deployment1",
			objectName:       []string{"deployment1"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:   5,
			expectedReplicas: 0,
		},
		{
			test:             "TestMultipleNamespacesMultipleDeployments",
			name:             "multiple namespaces and deployments. downscale deployments deployment2,deployment3",
			objectName:       []string{"deployment2", "deployment3"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:   5,
			expectedReplicas: 0,
		},
		{
			test:             "TestSingleNamespaceSingleDeploymentWithStatefulOverride",
			name:             "single namespace and deployment. downscale deployment deployment4 - should only try to downscale statefulset which means the deployment will be with 5 replicas",
			objectName:       []string{"deployment4"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas: []objecttypes.ResourceType{"statefulset"},
			initiaReplicas:   5,
			expectedReplicas: 5,
		},
		{
			test:             "TestMultipleNamespacesMultipleDeploymentsWithStatefulOverride",
			name:             "multiple namespaces and deployments. downscale deployments deployment5,deployment6 - should only try to downscale statefulset which means the deployments will be with 5 replicas",
			objectName:       []string{"deployment5", "deployment6"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas: []objecttypes.ResourceType{"statefulset"},
			initiaReplicas:   5,
			expectedReplicas: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientObjectList := createObjects(&appsv1.Deployment{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(time.Second, -1)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, nil)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.Deployment{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated deployment: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}
		})
	}
}

func TestDownscalingStatefulsets(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:             "TestSingleNamespaceSingleStatefulset",
			name:             "single namespace. downscale statefulset statefulset1",
			objectName:       []string{"statefulset1"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:   2,
			expectedReplicas: 0,
		},
		{
			test:             "TestMultipleNamespacesMultipleStatefulsets",
			name:             "multiple namespaces and statefulsets. downscale statefulsets statefulset2,statefulset3",
			objectName:       []string{"statefulset2", "statefulset3"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:   2,
			expectedReplicas: 0,
		},
		{
			test:             "TestSingleNamespaceSingleStatefulsetWithStatefulOverride",
			name:             "single namespace and statefulset. downscale statefulset statefulset4 - should only try to downscale deployments which means the statefulset will be with 2 replicas",
			objectName:       []string{"statefulset4"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas: []objecttypes.ResourceType{"deployments"},
			initiaReplicas:   2,
			expectedReplicas: 2,
		},
		{
			test:             "TestMultipleNamespacesMultipleStatefulsetsWithStatefulOverride",
			name:             "multiple namespaces and statefulsets. downscale statefulsets statefulset5,statefulset6 - should only try to downscale deployments which means the statefulsets will be with 2 replicas",
			objectName:       []string{"statefulset5", "statefulset6"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas: []objecttypes.ResourceType{"deployments"},
			initiaReplicas:   2,
			expectedReplicas: 2,
		},
	}

	for _, tc := range testCases {

		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientObjectList := createObjects(&appsv1.StatefulSet{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(time.Second, -1)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, nil)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated deployment: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}
		})
	}
}

func TestUpscalingDeployments(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:             "TestSingleNamespaceSingleDeployment",
			name:             "single namespace. upscale deployment deployment1",
			objectName:       []string{"deployment1"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:   0,
			expectedReplicas: 1,
		},
		{
			test:             "TestMultipleNamespacesMultipleDeployments",
			name:             "multiple namespaces and deployments. upscale deployments deployment2,deployment3",
			objectName:       []string{"deployment2", "deployment3"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:   0,
			expectedReplicas: 1,
		},
		{
			test:             "TestSingleNamespaceSingleDeploymentWithStatefulOverride",
			name:             "single namespace and deployment. upscale deployment deployment4 - should only try to upscale statefulset which means the deployment will be with 0 replicas",
			objectName:       []string{"deployment4"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas: []objecttypes.ResourceType{"statefulset"},
			initiaReplicas:   0,
			expectedReplicas: 0,
		},
		{
			test:             "TestMultipleNamespacesMultipleDeploymentsWithStatefulOverride",
			name:             "multiple namespaces and deployments. upscale deployments deployment5,deployment6 - should only try to upscale statefulset which means the deployments will be with 0 replicas",
			objectName:       []string{"deployment5", "deployment6"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas: []objecttypes.ResourceType{"statefulset"},
			initiaReplicas:   0,
			expectedReplicas: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientObjectList := createObjects(&appsv1.Deployment{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(-1, time.Second)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, nil)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.Deployment{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated deployment: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}
		})
	}
}

func TestUpscalingStatefulsets(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:             "TestSingleNamespaceSingleStatefulset",
			name:             "single namespace. upscale statefulset statefulset1",
			objectName:       []string{"statefulset1"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:   0,
			expectedReplicas: 1,
		},
		{
			test:             "TestMultipleNamespacesMultipleStatefulsets",
			name:             "multiple namespaces and statefulsets. upscale statefulsets statefulset2,statefulset3",
			objectName:       []string{"statefulset2", "statefulset3"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:   0,
			expectedReplicas: 1,
		},
		{
			test:             "TestSingleNamespaceSingleStatefulsetWithStatefulOverride",
			name:             "single namespace and statefulset. upscale statefulset statefulset4 - should only try to upscale deployments which means the statefulset will be with 0 replicas",
			objectName:       []string{"statefulset4"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas: []objecttypes.ResourceType{"deployments"},
			initiaReplicas:   0,
			expectedReplicas: 0,
		},
		{
			test:             "TestMultipleNamespacesMultipleStatefulsetsWithStatefulOverride",
			name:             "multiple namespaces and statefulsets. upscale statefulsets statefulset5,statefulset6 - should only try to upscale deployments which means the statefulsets will be with 0 replicas",
			objectName:       []string{"statefulset5", "statefulset6"},
			namespaces:       []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas: []objecttypes.ResourceType{"deployments"},
			initiaReplicas:   0,
			expectedReplicas: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientObjectList := createObjects(&appsv1.StatefulSet{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(-1, time.Second)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, nil)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated deployment: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}

		})
	}
}

func TestLifecycleSqlite(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:                       "TestSingleNamespaceSingleStatefulset",
			name:                       "single namespace. upscale statefulset statefulset1",
			objectName:                 []string{"statefulset1"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:             5,
			expectedDownscaledReplicas: 0,
			expectedReplicas:           5,
		},
		{
			test:                       "TestMultipleNamespacesMultipleStatefulsets",
			name:                       "multiple namespaces and statefulsets. upscale statefulsets statefulset2,statefulset3",
			objectName:                 []string{"statefulset2", "statefulset3"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:             5,
			expectedDownscaledReplicas: 0,
			expectedReplicas:           5,
		},
		{
			test:                       "TestSingleNamespaceSingleStatefulsetWithStatefulOverride",
			name:                       "single namespace and statefulset. upscale statefulset statefulset4 - should only try to upscale deployments which means the statefulset will be with 0 replicas",
			objectName:                 []string{"statefulset4"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas:           []objecttypes.ResourceType{"deployments"},
			initiaReplicas:             2,
			expectedDownscaledReplicas: 2,
			expectedReplicas:           2,
		},
		{
			test:                       "TestMultipleNamespacesMultipleStatefulsetsWithStatefulOverride",
			name:                       "multiple namespaces and statefulsets. upscale statefulsets statefulset5,statefulset6 - should only try to upscale deployments which means the statefulsets will be with 0 replicas",
			objectName:                 []string{"statefulset5", "statefulset6"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas:           []objecttypes.ResourceType{"deployments"},
			initiaReplicas:             2,
			expectedDownscaledReplicas: 2,
			expectedReplicas:           2,
		},
	}

	dbClient, err := sql.Open("sqlite", ":memory:")
	dbClient.SetMaxOpenConns(1)
	if err != nil {
		t.Fatalf("failed to connect to in memory db: %v", err)
	}

	storeClient := &store.Persistence{ScalingOperation: store.NewSqliteScalingOperationStore(dbClient)}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			clientObjectList := createObjects(&appsv1.StatefulSet{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(time.Second, time.Second*2)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, storeClient)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated statefulset: %v", err)
				}
				assert.Equal(t, tc.expectedDownscaledReplicas, *updatedObject.Spec.Replicas)
			}

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated statefulset: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}
		})
	}
}

func TestLifecyclePostgres(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	testCases := []testCase{
		{
			test:                       "TestSingleNamespaceSingleStatefulset",
			name:                       "single namespace. upscale statefulset statefulset1",
			objectName:                 []string{"statefulset1"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app1"},
			initiaReplicas:             5,
			expectedDownscaledReplicas: 0,
			expectedReplicas:           5,
		},
		{
			test:                       "TestMultipleNamespacesMultipleStatefulsets",
			name:                       "multiple namespaces and statefulsets. upscale statefulsets statefulset2,statefulset3",
			objectName:                 []string{"statefulset2", "statefulset3"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app2", "ns-app3"},
			initiaReplicas:             5,
			expectedDownscaledReplicas: 0,
			expectedReplicas:           5,
		},
		{
			test:                       "TestSingleNamespaceSingleStatefulsetWithStatefulOverride",
			name:                       "single namespace and statefulset. upscale statefulset statefulset4 - should only try to upscale deployments which means the statefulset will be with 0 replicas",
			objectName:                 []string{"statefulset4"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app4"},
			overrideReplicas:           []objecttypes.ResourceType{"deployments"},
			initiaReplicas:             2,
			expectedDownscaledReplicas: 2,
			expectedReplicas:           2,
		},
		{
			test:                       "TestMultipleNamespacesMultipleStatefulsetsWithStatefulOverride",
			name:                       "multiple namespaces and statefulsets. upscale statefulsets statefulset5,statefulset6 - should only try to upscale deployments which means the statefulsets will be with 0 replicas",
			objectName:                 []string{"statefulset5", "statefulset6"},
			namespaces:                 []downscalergov1alpha1.Namespace{"ns-app5", "ns-app6"},
			overrideReplicas:           []objecttypes.ResourceType{"deployments"},
			initiaReplicas:             2,
			expectedDownscaledReplicas: 2,
			expectedReplicas:           2,
		},
	}

	ctx := context.Background()

	const (
		postgresCredentials = "postgres"
		ctrImage            = "postgres:14.15-alpine3.20"
	)

	ctr, err := postgres.Run(ctx,
		ctrImage,
		postgres.WithDatabase(postgresCredentials),
		postgres.WithUsername(postgresCredentials),
		postgres.WithPassword(postgresCredentials),
		testcontainers.WithHostPortAccess(31555),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(1).
				WithStartupTimeout(10*time.Second),
			wait.ForExposedPort(),
		),
	)

	if err != nil {
		t.Fatalf("unexpected error while initializing db container: %v", err)
	}

	defer func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Log("error terminating the container: ", err)
		}
	}()

	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("error fetching container connection string: %v", err)
	}

	dbClient, err := sql.Open("postgres", conn)
	if err != nil {
		t.Fatalf("failed to connect to in memory db: %v", err)
	}

	if err := dbClient.Ping(); err != nil {
		t.Fatalf("db ping fail: %v", err)
	}

	defer dbClient.Close()

	storeClient := &store.Persistence{ScalingOperation: store.NewPostgresScalingOperationStore(dbClient)}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {

			clientObjectList := createObjects(&appsv1.StatefulSet{}, tc.namespaces, tc.objectName, tc.initiaReplicas)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjectList...).Build()
			c := apiclient.NewAPIClient(fakeClient)

			testDownscaleTime, testUpscaleTime := createTestScaleTime(time.Second, time.Second*2)
			downscalerObject := setupDownscalerObject(testDownscaleTime, testUpscaleTime, tc.name, tc.namespaces, tc.overrideReplicas)

			dm := intializeManager(t, c, downscalerObject, storeClient)
			defer dm.cron.Stop()

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated statefulset: %v", err)
				}
				assert.Equal(t, tc.expectedDownscaledReplicas, *updatedObject.Spec.Replicas)
			}

			<-time.After(oneSecond)

			for i := range tc.objectName {
				updatedObject := &appsv1.StatefulSet{}
				if err := c.Get(tc.namespaces[i].String(), updatedObject, tc.objectName[i]); err != nil {
					t.Fatalf("error getting updated statefulset: %v", err)
				}
				assert.Equal(t, tc.expectedReplicas, *updatedObject.Spec.Replicas)
			}
		})
	}
}
