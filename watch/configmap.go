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

func ConfigMap(ctx context.Context, name, namespace string, client k8sutil.KubernetesHelper, cmObjectch chan runtime.Object) {
	for {
		watcher, err := client.GetWatcherByConfigMapName(
			ctx,
			name,
			namespace,
		)
		if err != nil {
			slog.Error("error initializing a new configmap watcher", "next retry", "10 seconds", "error", err.Error())
			time.Sleep(time.Second * 10)
			continue
		}

	createNewWatcher:
		for {
			event, open := <-watcher.ResultChan()
			if !open {
				watcher.Stop()
				slog.Warn("watcher closed, restarting...", "reason", "channel closed")
				break createNewWatcher
			}
			switch event.Type {
			case watch.Modified:
				cm := event.Object.(*corev1.ConfigMap)
				slog.Info("configmap was updated",
					"name", cm.Name,
					"namespace", cm.Namespace,
				)
				cmObjectch <- event.Object
			case watch.Error:
				slog.Error("error updating the object",
					"resource type", "configmap",
				)
			}
		}
	}
}
