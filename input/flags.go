package input

import (
	"flag"
	"log/slog"
	"os"
)

type FromFlags struct {
	InitialCmConfig string
}

func Flags() *FromFlags {
	InitialCmPath := flag.String("configmap_name", "", "set the configmap name")
	flag.Parse()
	if *InitialCmPath == "" {
		slog.Error("validate deployment args", "error", "missing configmap name")
		os.Exit(1)
	}
	return &FromFlags{InitialCmConfig: *InitialCmPath}
}
