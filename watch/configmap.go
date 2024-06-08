package watch

import (
	"log/slog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func ConfigMap(watcher watch.Interface, cmObjectch chan runtime.Object) {
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Modified:
			slog.Info("received new modified object", "resourceType", "configMap")
			cmObjectch <- event.Object
		case watch.Error:
			slog.Error("error updating the object", "resourceType", "configMap")
		}
	}
}
