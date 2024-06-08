package watch

import (
	"context"
	"log/slog"
	"time"

	"github.com/adalbertjnr/downscaler/k8sutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func ConfigMap(ctx context.Context, client k8sutil.KubernetesHelper, watcher watch.Interface, cmObjectch chan runtime.Object) {
	for {
		select {
		case event, received := <-watcher.ResultChan():
			if !received {
				slog.Warn("watcher close, restarting...")
				watcher.Stop()
				break
			}
			cm := event.Object.(*corev1.ConfigMap)
			slog.Info("configmap was updated",
				"name", cm.Name,
				"namespace", cm.Namespace,
			)
			cmObjectch <- event.Object
			if event.Type == watch.Error {
				slog.Error("error updating the object",
					"resource type", "configmap",
				)
			}
		case <-time.After(time.Second * 35):
			watcher, err := client.GetWatcherByConfigMapName(ctx, *cmName, *cmNamespace)
			if err != nil {
				slog.Error("error renewing the configmap watcher", "error", err.Error())
			}
		}
	}
}

// func ConfigMap(watcher watch.Interface, cmObjectch chan runtime.Object) {
// 	for event := range watcher.ResultChan() {
// 		switch event.Type {
// 		case watch.Modified:
// 			cm := event.Object.(*corev1.ConfigMap)
// 			slog.Info("configmap was updated",
// 				"name", cm.Name,
// 				"namespace", cm.Namespace,
// 			)
// 			cmObjectch <- event.Object
// 		case watch.Error:
// 			slog.Error("error updating the object",
// 				"resource type", "configmap",
// 			)
// 		}
// 	}
// }
