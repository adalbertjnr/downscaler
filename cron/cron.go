package cron

import (
	"time"

	"github.com/adalbertjnr/downscaler/types"
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

func (c *Cron) MustAddLocation(timeZone string) *Cron {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		panic(err)
	}
	c.Location = location
	return c
}

func (c *Cron) AddYamlPolicy(downscalerCm *types.DownscalerPolicy) {
	c.parseCronInput()
}

func (c *Cron) parseCronInput() {

}

func (c *Cron) StartCron() *Cron {
	go func() {
		for {
		}
	}()

	return c
}
