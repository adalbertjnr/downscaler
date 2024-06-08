package watch

import (
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func ConfigMap(watcher watch.Interface, cmObjectch chan runtime.Object) {
	for event := range watcher.ResultChan() {
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
