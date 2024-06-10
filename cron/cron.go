package cron

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/shared"
)

const (
	ExpectedTimeParts = 2
	TimeFormat        = "15:04"
)

type Criteria struct {
	Namespaces []string
	WithCron   string
	Recurrence string
}

type CronTask struct {
	Criteria
}

type Cron struct {
	Kubernetes         k8sutil.KubernetesHelper
	Location           *time.Location
	Tasks              []CronTask
	ExcludedNamespaces []string
	Recurrence         string
	taskch             chan []CronTask
	stopch             chan struct{}
	taskRoutines       map[string]chan struct{}
}

func NewCron() *Cron {
	return &Cron{
		taskch:       make(chan []CronTask, 1),
		stopch:       make(chan struct{}),
		taskRoutines: make(map[string]chan struct{}),
	}
}

func (c *Cron) AddKubeApiSvc(client k8sutil.KubernetesHelper) *Cron {
	c.Kubernetes = client
	return c
}

func (c *Cron) AddCronDetails(downscalerData *shared.DownscalerPolicy) {
	if errors := c.Validate(downscalerData); len(errors) > 0 {
		for _, err := range errors {
			slog.Error("crontime validator", "error", err)
		}
		return
	}

	c.updateRecurrenceIfEmpty(downscalerData.Spec.ExecutionOpts.Time.Recurrence)
	c.parseCronConfig(downscalerData)
}

func (c *Cron) parseCronConfig(downscalerData *shared.DownscalerPolicy) {
	if still := stillSameRecurrenceTime(
		downscalerData.Spec.ExecutionOpts.Time.Recurrence,
		c.Recurrence,
	); !still {
		c.Recurrence = downscalerData.Spec.ExecutionOpts.Time.Recurrence
		slog.Info("cron recurrence received", "recurrence", c.Recurrence)
	}

	expression := ParseExpr(downscalerData)
	if expression.withExclude() {
		namespaces := expression.MatchExpressions.Values
		c.ExcludedNamespaces = append([]string{}, namespaces...)
	}

	criteria := ParseCriteria(downscalerData)
	if criteria.available() {
		tasks := make([]CronTask, len(criteria.Criteria))

		for i, crit := range criteria.Criteria {
			tasks[i] = CronTask{
				Criteria: Criteria{
					Namespaces: crit.Namespaces,
					WithCron:   crit.WithCron,
					Recurrence: c.Recurrence,
				},
			}
			fmt.Println("crontask: ", crit.WithCron, crit.Namespaces)
		}
		c.taskch <- tasks
	}
}

func (c *Cron) StartCron() {
	c.runCronLoop()
}

func (c *Cron) runCronLoop() {
	for {
		if tasks, open := <-c.taskch; open {
			c.updateTasks(tasks)
		}
		time.Sleep(time.Second * 5)
		continue
	}
}

func (c *Cron) updateTasks(tasks []CronTask) {
	newTaskMap := make(map[string]CronTask)

	for _, task := range tasks {
		key := task.Criteria.WithCron + strings.Join(task.Namespaces, ",")
		newTaskMap[key] = task

		if _, exists := c.taskRoutines[key]; !exists {
			c.killCurrentCronRoutines()

			stopch := make(chan struct{})
			c.taskRoutines[key] = stopch

			go c.runTasks(task, stopch)
			slog.Info("changes detected in cron time",
				"action", "triggering new routine",
				"recurrence", task.Recurrence,
				"namespaces", task.Namespaces,
				"crontime", task.WithCron,
			)
			return
		}
		slog.Info("no changes detected in cron time", "action", "ignoring")
	}
}

func (c *Cron) runTasks(task CronTask, stopch chan struct{}) {
	var (
		from, until = getCronFromAndUntil(task.WithCron)
		namespaces  = task.Namespaces
		recurrence  = task.Recurrence
		ctx         = context.Background()
	)

	recurrenceDays := parseRecurrence(recurrence)
	_ = from

	for {
		select {
		case <-stopch:
			slog.Info("crontask routine is been terminated", "reason", "recycling due the new crontime changes")
			return
		default:
			now := time.Now().In(c.Location)
			if !c.isRecurrenceDay(now.Weekday(), recurrenceDays) {
				time.Sleep(time.Hour * 1)
				slog.Info("time", "today is ", now.Weekday().String(), "recurrence days range", recurrenceDays,
					"action", "waiting",
				)
				continue
			}

			if now.After(until) {
				if valid := c.validateCronNamespaces(ctx, namespaces); !valid {
					time.Sleep(time.Minute * 1)
					continue
				}

				slog.Info("initializing downscaling process", "recurrence", recurrence,
					"namespaces", namespaces,
				)
				k8sutil.TriggerDownscaler(ctx, c.Kubernetes, namespaces)
			}
		}
	}

}

func (c *Cron) killCurrentCronRoutines() {
	if len(c.taskRoutines) > 0 {
		slog.Info("cleaning crontask map", "reason", "crontime update received")
		for key, stopch := range c.taskRoutines {
			slog.Info("cleaning crontask map", "key", key)
			delete(c.taskRoutines, key)
			close(stopch)
		}
	}
}

func (c *Cron) MustAddTimezoneLocation(timeZone string) *Cron {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		panic(err)
	}
	slog.Info("received timezone from the config", "timezone", timeZone)
	c.Location = location
	return c
}

func (c *Cron) isRecurrenceDay(day time.Weekday, recurrenceDays []time.Weekday) bool {
	for _, recurrenceDay := range recurrenceDays {
		if day == recurrenceDay {
			return true
		}
	}
	return false
}

func parseRecurrence(recurrence string) []time.Weekday {
	days := map[string]time.Weekday{
		"MON": time.Monday,
		"TUE": time.Tuesday,
		"WED": time.Wednesday,
		"THU": time.Thursday,
		"FRI": time.Friday,
		"SAT": time.Saturday,
		"SUN": time.Sunday,
	}
	recurrenceDays := []time.Weekday{}

	for _, day := range strings.Split(recurrence, "-") {
		if weekday, found := days[day]; found {
			recurrenceDays = append(recurrenceDays, weekday)
		}
	}
	return recurrenceDays
}

func (c *Cron) updateRecurrenceIfEmpty(recurrence string) {
	if c.Recurrence == "" {
		c.Recurrence = recurrence
	}
}

func getCronFromAndUntil(rawTime string) (from, until time.Time) {
	var err error
	parts := strings.SplitN(rawTime, "-", ExpectedTimeParts)
	if len(parts) != ExpectedTimeParts {
		slog.Error("error parsing cron time", "time received", rawTime)
		return
	}
	from, err = time.Parse(TimeFormat, parts[0])
	if err != nil {
		slog.Error("error parsing the time from", "time", parts[0])
		return
	}
	until, err = time.Parse(TimeFormat, parts[1])
	if err != nil {
		slog.Error("error parsing the time until", "time", parts[1])
		return
	}

	return from, until
}
