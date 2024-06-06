package cron

type Cron struct{}

func NewCron() *Cron {
	return &Cron{}
}

func (c *Cron) AddCron(cron string) {

	c.parseCronInput()
}

func (c *Cron) parseCronInput() {

}
