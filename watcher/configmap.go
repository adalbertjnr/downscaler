package watcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type Watcher struct {
	RtObjectch chan runtime.Object
}

func New() *Watcher {
	return &Watcher{
		RtObjectch: make(chan runtime.Object),
	}
}

func (w *Watcher) ConfigMap(ctx context.Context, metadata shared.Metadata, client k8sutil.KubernetesHelper) {
	for {
		watcher, err := client.GetWatcherByConfigMapName(
			ctx,
			metadata.Name,
			metadata.Namespace,
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
				slog.Warn("watcher closed", "reason", "recycling")
				break createNewWatcher
			}
			switch event.Type {
			case watch.Modified:
				cm := event.Object.(*corev1.ConfigMap)
				slog.Info("configmap was updated",
					"name", cm.Name,
					"namespace", cm.Namespace,
				)
				w.RtObjectch <- event.Object
			case watch.Error:
				slog.Error("error updating the object",
					"resource type", "configmap",
				)
			}
		}
	}
}
