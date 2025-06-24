/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

<<<<<<< HEAD
	downscalergov1alpha1 "github.com/adalbertjnr/downscaler/api/v1alpha1"
	"github.com/adalbertjnr/downscaler/internal/client"
	"github.com/adalbertjnr/downscaler/internal/db"
	"github.com/adalbertjnr/downscaler/internal/factory"
	"github.com/adalbertjnr/downscaler/internal/manager"
	"github.com/adalbertjnr/downscaler/internal/store"
	objecttypes "github.com/adalbertjnr/downscaler/internal/types"
	"github.com/adalbertjnr/downscaler/internal/utils"
=======
	downscalergov1alpha1 "github.com/adalbertjnr/kubetime-scaler/api/v1alpha1"
	"github.com/adalbertjnr/kubetime-scaler/internal/client"
	"github.com/adalbertjnr/kubetime-scaler/internal/db"
	"github.com/adalbertjnr/kubetime-scaler/internal/factory"
	"github.com/adalbertjnr/kubetime-scaler/internal/manager"
	"github.com/adalbertjnr/kubetime-scaler/internal/store"
	objecttypes "github.com/adalbertjnr/kubetime-scaler/internal/types"
	"github.com/adalbertjnr/kubetime-scaler/internal/utils"
>>>>>>> main
)

var _ = Describe("Downscaler Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			namespace    = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
		}

		downscaler := &downscalergov1alpha1.Downscaler{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Downscaler")
			err := k8sClient.Get(ctx, typeNamespacedName, downscaler)
			if err != nil && errors.IsNotFound(err) {
				resource := &downscalergov1alpha1.Downscaler{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "downscaler.go/v1alpha1",
						Kind:       "Downscaler",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: downscalergov1alpha1.DownscalerSpec{
						Config:   downscalergov1alpha1.Config{CronLoggerInterval: 60},
						Schedule: downscalergov1alpha1.Schedule{TimeZone: "America/Sao_Paulo", Recurrence: "@daily"},
						DownscalerOptions: downscalergov1alpha1.DownscalerOptions{
							ResourceScaling: []objecttypes.ResourceType{"deployments", "statefulset"},
							TimeRules: &downscalergov1alpha1.TimeRules{
								Rules: []downscalergov1alpha1.Rules{
									{
										Name:            "Rule to test",
										Namespaces:      []downscalergov1alpha1.Namespace{"test1", "test2"},
										DownscaleTime:   "15:30",
										UpscaleTime:     "15:40",
										OverrideScaling: []objecttypes.ResourceType{"deployments", "statefulset"},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &downscalergov1alpha1.Downscaler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Downscaler")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			apiClient := client.NewAPIClient(k8sClient)
			dbConfig := db.Config{
				Driver: utils.LookupString(os.Getenv("DB_DRIVER"), "memory_store"),
				DSN:    utils.LookupString(os.Getenv("DSN"), ""),
			}
			storeClient := store.New(logr.Logger{}, false, dbConfig)
			scalerFactory := factory.NewScalerFactory(apiClient, storeClient, logr.Logger{})

			downscalerScheduler := (&manager.Downscaler{}).
				Client(apiClient).
				Factory(scalerFactory).
				Persistence(storeClient).
				Logger(logr.Logger{})

			By("Reconciling the created resource")
			controllerReconciler := &DownscalerReconciler{
				Client:              k8sClient,
				Scheme:              k8sClient.Scheme(),
				DownscalerScheduler: downscalerScheduler,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
