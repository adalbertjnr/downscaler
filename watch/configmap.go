package watch

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func ConfigMap(watcher watch.Interface, cmObjectch chan runtime.Object) {
	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Modified:
			fmt.Println("configMap modified")
			cmObjectch <- event.Object
		}
	}
}
