package helpers

import (
	"log/slog"
	"os"
)

type Helpers struct{}

func New() *Helpers {
	return &Helpers{}
}

func (h *Helpers) CurrentNamespace() string {
	namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	slog.Info("found current namespace", "namespace", string(namespace))
	return string(namespace)
}
