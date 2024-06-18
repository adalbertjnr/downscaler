package input

import (
	"flag"
)

type FromFlags struct {
	ConfigMapName      string
	ConfigMapNamespace string
}

func Flags() *FromFlags {
	configMapName := flag.String("configmap_name", "downscaler-cm", "set the configmap name")
	configMapNamespace := flag.String("configmap_namespace", "downscaler", "set the configmap namespace")
	flag.Parse()
	return &FromFlags{ConfigMapName: *configMapName, ConfigMapNamespace: *configMapNamespace}
}
