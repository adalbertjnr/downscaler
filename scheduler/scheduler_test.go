package scheduler

import (
	"testing"
	"time"

	"github.com/adalbertjnr/downscaler/shared"
)

func TestBeforeDownscalingValidation(t *testing.T) {
	var (
		now                            = time.Now()
		location                       = now.Location()
		upscalingTime, downscalingTime = extractUpscalingAndDownscalingTime("06:00-22:00", location)
	)

	tests := []struct {
		name     string
		state    shared.TaskControl
		testTime time.Time
		expected bool
	}{
		{"With no data write (first init, before midnight and before downscaling time)", shared.AppStartupWithNoDataWrite, time.Date(now.Year(), now.Month(), now.Day(), 18, 20, 0, 0, location), true},
		{"With no data write (first init, before midnight and after downscaling time)", shared.AppStartupWithNoDataWrite, time.Date(now.Year(), now.Month(), now.Day(), 23, 50, 0, 0, location), false},
		{"With downscaledState (already init but the app did break in some moment before midnight)", shared.DeploymentsWithDownscaledState, time.Date(now.Year(), now.Month(), now.Day(), 23, 50, 0, 0, location), false},
		{"With downscaledState (already init but the app did break in some moment after midnight but before upscaleTime)", shared.DeploymentsWithDownscaledState, time.Date(now.Year(), now.Month(), now.Day(), 05, 50, 0, 0, location), false},
		{"With upscaledState (already init but the app did break in some moment before midnight)", shared.DeploymentsWithUpscaledState, time.Date(now.Year(), now.Month(), now.Day(), 07, 50, 0, 0, location), true},
		{"With upscaledState (after downscaled state but it's going to downscale again in 10 minutes)", shared.DeploymentsWithUpscaledState, time.Date(now.Year(), now.Month(), now.Day(), 21, 50, 0, 0, location), true},
		{"With upscaledState (after downscaled state but it's going to downscale again now)", shared.DeploymentsWithUpscaledState, time.Date(now.Year(), now.Month(), now.Day(), 22, 01, 0, 0, location), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validateIfShouldRunDownscalingOrWait(tt.testTime, tt.state, downscalingTime, upscalingTime)
			if resp != tt.expected {
				t.Errorf("validateIfShouldRunDownscalingOrWait(%v) = %v; expected %v -> now(%v), down(%v), upsc(%v)", tt.testTime, resp, tt.expected, tt.testTime, downscalingTime, upscalingTime)
			}
		})
	}
}

func TestAfterDownscalingValidation(t *testing.T) {
	var (
		now                            = time.Now()
		location                       = now.Location()
		upscalingTime, downscalingTime = extractUpscalingAndDownscalingTime("06:00-22:00", location)
	)

	tests := []struct {
		name     string
		testTime time.Time
		expected bool
	}{
		{"With downscaledState after downscaled and before midnight", time.Date(now.Year(), now.Month(), now.Day(), 23, 20, 0, 0, location), true},
		{"With downscaledState after downscaled and after midnight", time.Date(now.Year(), now.Month(), now.Day(), 05, 20, 0, 0, location), true},
		{"With downscaledState after downscaled and after upscaling time", time.Date(now.Year(), now.Month(), now.Day(), 06, 05, 0, 0, location), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validateIfShoudRunUpscalingOrWait(tt.testTime, upscalingTime, downscalingTime)
			if resp != tt.expected {
				t.Errorf("validateIfShouldRunUpscalingOrWait(%v) = %v; expected %v -> now(%v), down(%v), upsc(%v)", tt.testTime, resp, tt.expected, tt.testTime, downscalingTime, upscalingTime)
			}
		})
	}
}
