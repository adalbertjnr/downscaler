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
	Namespaces   []string
	WithCron     string
	Recurrence   string
	CriteriaList map[string]struct{}
}

type CronTask struct {
	Criteria
}

type Cron struct {
	Kubernetes        k8sutil.KubernetesHelper
	Location          *time.Location
	Tasks             []CronTask
	IgnoredNamespaces map[string]struct{}
	Recurrence        string
	taskch            chan []CronTask
	stopch            chan struct{}
	taskRoutines      map[string]chan struct{}
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

	var (
		expression = downscalerData.Spec.ExecutionOpts.Time.Downscaler.DownscalerSelectorTerms.MatchExpressions
		criteria   = downscalerData.Spec.ExecutionOpts.Time.Downscaler.WithAdvancedNamespaceOpts.MatchCriteria.Criteria
		recurrence = downscalerData.Spec.ExecutionOpts.Time.Recurrence
	)

	c.updateRecurrenceIfEmpty(recurrence)
	c.parseCronConfig(
		recurrence,
		DownscalerExpression{MatchExpressions: expression},
		DownscalerCriteria{Criteria: criteria},
	)

}

func (c *Cron) parseCronConfig(
	recurrence string,
	expression DownscalerExpression,
	criteria DownscalerCriteria,
) {

	if still := stillSameRecurrenceTime(
		recurrence,
		c.Recurrence,
	); !still {
		c.Recurrence = recurrence
		slog.Info("cron recurrence received", "recurrence", c.Recurrence)
	}

	if expression.withExclude() {
		expressionValues := expression.MatchExpressions.Values
		excludedNamespaces := make([]string, len(expressionValues))
		excludedNamespaces = append(excludedNamespaces, expressionValues...)
		ignoredNamespaces := expression.showIgnoredNamespaces(excludedNamespaces)

		c.ignoredNamespacesCleanupValidation(ignoredNamespaces)
	}

	if criteria.available() {
		tasks := make([]CronTask, len(criteria.Criteria))

		scheduledNamespaces := separatedScheduledNamespaces(criteria)

		for i, crit := range criteria.Criteria {
			tasks[i] = CronTask{
				Criteria: Criteria{
					Namespaces:   crit.Namespaces,
					WithCron:     crit.WithCron,
					Recurrence:   c.Recurrence,
					CriteriaList: scheduledNamespaces,
				},
			}
		}
		c.killCurrentCronRoutines()
		c.taskch <- tasks
	}
}

func separatedScheduledNamespaces(criterias DownscalerCriteria) map[string]struct{} {
	scheduledNamespaces := make([]string, 0)
	for _, criteria := range criterias.Criteria {
		scheduledNamespaces = append([]string{}, criteria.Namespaces...)
		continue
	}
	return generateScheduledNamespaces(scheduledNamespaces)
}

func generateScheduledNamespaces(scheduledNamespaces []string) map[string]struct{} {
	scheduledNamespacesMap := make(map[string]struct{}, len(scheduledNamespaces))
	for _, namespace := range scheduledNamespaces {
		if _, exists := scheduledNamespacesMap[namespace]; !exists {
			scheduledNamespacesMap[namespace] = struct{}{}
		}
	}
	return scheduledNamespacesMap
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

			stopch := make(chan struct{})
			c.taskRoutines[key] = stopch

			go c.runTasks(task, stopch)
			slog.Info("changes detected in crontime",
				"action", "triggering task routine",
				"recurrence", task.Recurrence,
				"namespaces", task.Namespaces,
				"crontime", task.WithCron,
			)
		} else {
			slog.Info("no changes detected in cron time", "action", "ignoring")
		}
	}
}

func (c *Cron) runTasks(task CronTask, stopch chan struct{}) {
	slog.Info("routine",
		"with namespace(s) task", task.Namespaces,
		"with cron(s) task", task.WithCron,
		"with recurrence", task.Recurrence,
		"status", "initializing",
	)

	defer func() {
		slog.Info("routine",
			"with namespace(s) task", task.Namespaces,
			"with cron(s) task", task.WithCron,
			"with recurrence", task.Recurrence,
			"status", "terminated",
			"reason", "crontime updated",
		)
	}()

	var (
		from, until = fromUntil(task.WithCron, c.Location)
		namespaces  = task.Namespaces
		recurrence  = task.Recurrence
		ctx         = context.Background()
	)

	recurrenceDays := parseRecurrence(recurrence)
crontask:
	for {
		select {
		case <-stopch:
			break crontask
		default:
			now := time.Now().In(c.Location)
			if !c.isRecurrenceDay(now.Weekday(), recurrenceDays) {
				slog.Info("time",
					"today is", now.Weekday().String(),
					"recurrence days range", recurrenceDays,
					"action", "waiting",
					"next try", "1 minute",
				)
				time.Sleep(time.Minute * 1)
				continue
			}

			if valid := c.validateCronNamespaces(ctx, namespaces); !valid {
				time.Sleep(time.Minute * 1)
				continue
			}

			if !now.After(until) {
				ut := fmt.Sprintf("%02d:%02d", until.Hour(), until.Minute())
				nw := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
				slog.Info("crontime",
					"provided crontime", ut,
					"current time", nw,
					"next retry", "1 minute",
				)
				time.Sleep(time.Minute * 1)
				continue
			}

			slog.Info("initializing downscaling process",
				"recurrence", recurrence,
				"namespaces", namespaces,
			)

			k8sutil.TriggerDownscaler(ctx,
				c.Kubernetes,
				namespaces,
				c.IgnoredNamespaces,
				task.CriteriaList,
			)

			nextRun := time.Until(from.Add(20 * time.Hour))
			time.Sleep(nextRun)
		}
	}

}

func (c *Cron) killCurrentCronRoutines() {
	if len(c.taskRoutines) > 0 {
		for key, stopch := range c.taskRoutines {
			slog.Info("cleaning crontask map", "key", key, "reason", "crontime updated")
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

	if len(recurrenceDays) == 2 {
		start, end := recurrenceDays[0], recurrenceDays[1]

		if start < end {
			for day := start + 1; day < end; day++ {
				recurrenceDays = append(recurrenceDays, day)
			}
		} else {
			for day := start + 1; day <= time.Saturday; day++ {
				recurrenceDays = append(recurrenceDays, day)
			}
			for day := time.Sunday; day < end; day++ {
				recurrenceDays = append(recurrenceDays, day)
			}
		}
	}
	return recurrenceDays
}

func (c *Cron) updateRecurrenceIfEmpty(recurrence string) {
	if c.Recurrence == "" {
		c.Recurrence = recurrence
	}
}

func fromUntil(rawTime string, loc *time.Location) (from, until time.Time) {
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

	now := time.Now().In(loc)
	from = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		from.Hour(),
		from.Minute(),
		0,
		0,
		loc,
	)

	until = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		until.Hour(),
		until.Minute(),
		0,
		0,
		loc,
	)

	return from, until
}
