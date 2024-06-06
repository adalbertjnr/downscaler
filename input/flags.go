package input

import "flag"

type FromFlags struct {
	InitialCmConfig string
}

func Flags() *FromFlags {
	InitialCmPath := flag.String("initial_config_path", "config/configmap.yaml", "set the initial configmap config path")
	flag.Parse()
	return &FromFlags{InitialCmConfig: *InitialCmPath}
}
