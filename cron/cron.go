package cron

import (
	"log/slog"
	"time"

	"github.com/adalbertjnr/downscaler/shared"
)

type Criteria struct {
	Namespace []string
	WithCron  string
}

type Cron struct {
	Hour     time.Time
	Minute   time.Time
	Location *time.Location
	C        Criteria
}

func NewCron() *Cron {
	return &Cron{}
}

func (c *Cron) MustAddTimezoneLocation(timeZone string) *Cron {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		panic(err)
	}
	slog.Info("received timezone from the config", "timeZone", timeZone)
	c.Location = location
	return c
}

func (c *Cron) AddYamlPolicy(downscalerCm *shared.DownscalerPolicy) {
	slog.Info("new cronjob received")
	c.parseCronInput()
}

func (c *Cron) parseCronInput() {

}

func (c *Cron) StartCron() {
}
