package helpers

import (
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
)

func (h *Helpers) Timezone(data *shared.DownscalerPolicy) string {
	return strings.TrimSpace(data.Spec.ExecutionOpts.Time.TimeZone)
}
