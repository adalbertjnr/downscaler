package watcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/adalbertjnr/downscaler/internal/common"
	"github.com/adalbertjnr/downscaler/kas"
	"github.com/adalbertjnr/downscaler/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (w *Watcher) DownscalerKind(
	ctx context.Context,
	metadata shared.Metadata,
	client kas.Kubernetes,
) {

	for {
		watcher, err := client.GetWatcherByDownscalerCRD(
			ctx,
			metadata.Name,
			metadata.Namespace,
		)
		if err != nil {
			slog.Error("error initializing a new configmap watcher", "next retry", "10 seconds", "error", err)
			time.Sleep(time.Second * 10)
			continue
		}

	createNewWatcher:
		for {
			event, open := <-watcher.ResultChan()
			if !open {
				watcher.Stop()
				slog.Warn("watcher", "status", "closed", "reason", "recycling due to timeout seconds")
				break createNewWatcher
			}
			switch event.Type {
			case watch.Modified:
				downscalerData := event.Object.(*unstructured.Unstructured)
				data := &shared.DownscalerPolicy{}
				if err := common.UnmarshalDataPolicy(downscalerData, data); err != nil {
					slog.Error("unmarshaling", "error unmarshaling in the watcher", err)
				}
				slog.Info("watcher", "kind", "downscaler", "name", data.Metadata.Name, "status", "updated")
				w.RtObjectch <- event.Object
			case watch.Error:
				slog.Error("error updating the object", "resource type", "Downscaler")
			}
		}
	}
}
