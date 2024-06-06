package cron

type Cron struct{}

func NewCron() *Cron {
	return &Cron{}
}

func (c *Cron) AddCron() {

	c.parseCronInput()
}

func (c *Cron) parseCronInput() {

}
