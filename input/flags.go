package input

import (
	"flag"
)

type FromArgs struct {
	ConfigMapName      string
	ConfigMapNamespace string
	RunUpscaling       bool
}

func FromEntrypoint() *FromArgs {
	runUpscaling := flag.Bool("run_upscaling", false, "set true if should run upscaling")
	configMapName := flag.String("configmap_name", "downscaler-cm", "set the configmap name")
	configMapNamespace := flag.String("configmap_namespace", "downscaler", "set the configmap namespace")
	flag.Parse()
	return &FromArgs{
		RunUpscaling:       *runUpscaling,
		ConfigMapName:      *configMapName,
		ConfigMapNamespace: *configMapNamespace,
	}
}
