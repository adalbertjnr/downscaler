package cron

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	k8s "github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/shared"
)

const (
	ExpectedTimeParts = 2
	TimeFormat        = "15:04"
)

type Rules struct {
	Namespaces          []string
	WithCron            string
	Recurrence          string
	ScheduledNamespaces map[string]struct{}
}

type CronTask struct {
	Rules
}

type Cron struct {
	Kubernetes        k8s.Kubernetes
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

func (c *Cron) AddKubeApiSvc(client k8s.Kubernetes) *Cron {
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
		rules      = downscalerData.Spec.ExecutionOpts.Time.Downscaler.WithNamespaceOpts.DownscaleNamespacesWithTimeRules.Rules
		recurrence = downscalerData.Spec.ExecutionOpts.Time.Recurrence
		timezone   = downscalerData.Spec.ExecutionOpts.Time.TimeZone
	)

	c.updateRecurrenceIfEmpty(recurrence)
	if err := c.updateTimeZoneIfNotEqual(timezone); err != nil {
		return
	}

	c.parseCronConfig(
		recurrence,
		DownscalerExpression{MatchExpressions: expression},
		DownscalerRules{Rules: rules},
	)

}

func (c *Cron) parseCronConfig(
	recurrence string,
	expression DownscalerExpression,
	rules DownscalerRules,
) {

	if !stillSameRecurrenceTime(recurrence, c.Recurrence) {
		c.Recurrence = recurrence
		slog.Info("cron recurrence received", "recurrence", c.Recurrence)
	}

	if expression.withExclude() {
		expressionValues := expression.MatchExpressions.Values
		excludedNamespaces := append([]string{}, expressionValues...)
		ignoredNamespaces := expression.showIgnoredNamespaces(excludedNamespaces)

		c.ignoredNamespacesCleanupValidation(ignoredNamespaces)
	}

	if rules.available() {
		tasks := make([]CronTask, len(rules.Rules))

		scheduledNamespaces := separatedScheduledNamespaces(rules)
		for i, crit := range rules.Rules {
			tasks[i] = CronTask{
				Rules: Rules{
					Namespaces:          crit.Namespaces,
					WithCron:            crit.WithCron,
					Recurrence:          c.Recurrence,
					ScheduledNamespaces: scheduledNamespaces,
				},
			}
		}
		c.killCurrentCronRoutines()
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
		key := task.Rules.WithCron + strings.Join(task.Namespaces, ",")
		newTaskMap[key] = task

		if _, exists := c.taskRoutines[key]; !exists {

			stopch := make(chan struct{})
			c.taskRoutines[key] = stopch

			go c.runTasks(task, stopch)
			slog.Info("changes detected in crontime", "action", "triggering task routine",
				"recurrence", task.Recurrence, "namespaces", task.Namespaces, "crontime", task.WithCron,
			)
		} else {
			slog.Info("no changes detected in cron time", "action", "ignoring")
		}
	}
}

func (c *Cron) runTasks(task CronTask, stopch chan struct{}) {
	slog.Info("crontask routine", "with namespace(s) task", task.Namespaces, "with cron(s) task", task.WithCron,
		"with recurrence", task.Recurrence, "reason", "crontime created", "status", "initializing",
	)

	defer func() {
		slog.Info("crontask routine", "with namespace(s) task", task.Namespaces, "with cron(s) task", task.WithCron,
			"with recurrence", task.Recurrence, "reason", "crontime updated", "status", "terminated",
		)
	}()

	var (
		ctx            = context.Background()
		namespaces     = task.Namespaces
		recurrence     = task.Recurrence
		recurrenceDays = parseRecurrence(recurrence)
	)

crontask:
	for {
		select {
		case <-stopch:
			break crontask
		default:
			_, until := fromUntil(task.WithCron, c.Location)
			nowBeforeScheduling := time.Now().In(c.Location)

			if !c.isRecurrenceDay(nowBeforeScheduling.Weekday(), recurrenceDays) {
				slog.Info("time", "today is", nowBeforeScheduling.Weekday().String(), "recurrence days range", "false",
					"action", "waiting", "next try", "1 minute",
				)
				time.Sleep(time.Minute * 1)
				continue
			}

			if valid := c.validateCronNamespaces(ctx, namespaces); !valid {
				time.Sleep(time.Minute * 1)
				continue
			}

			if !nowBeforeScheduling.After(until) {
				ut, nw := toStringWithFormat(until, nowBeforeScheduling)
				slog.Info("crontask routine", "current time", nw, "provided crontime", ut,
					"namespace(s)", namespaces, "status", "before downscaling", "next retry", "1 minute",
				)
				time.Sleep(time.Minute * 1)
				continue
			}

			notUsableNamespaces := shared.NotUsableNamespacesDuringScheduling{
				IgnoredNamespaces:   c.IgnoredNamespaces,
				ScheduledNamespaces: task.ScheduledNamespaces,
			}

			c.Kubernetes.StartDownscaling(ctx, namespaces, notUsableNamespaces)

		restartCronTask:
			for {
				select {
				case <-stopch:
					break crontask
				default:
					from, until := fromUntil(task.WithCron, c.Location)
					nowAfterScheduling := time.Now().In(c.Location)

					if (from.Before(until) && (nowAfterScheduling.Before(from) || nowAfterScheduling.After(until))) ||
						(from.After(until) && (nowAfterScheduling.Before(from) && nowAfterScheduling.After(until))) {
						fr, nw := toStringWithFormat(from, nowAfterScheduling)
						slog.Info("crontask routine", "current time", nw, "provided crontime", fr,
							"namespace(s)", namespaces, "status", "after downscaling", "next retry", "1 minute",
						)
						time.Sleep(time.Minute * 1)
						continue
					}
					break restartCronTask
				}
			}
		}
	}

}

func (c *Cron) killCurrentCronRoutines() {
	if len(c.taskRoutines) > 0 {
		for key, stopch := range c.taskRoutines {
			slog.Info("cleanup crontask map process",
				"key", key, "reason", "crontime updated by the user")
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

func (c *Cron) updateRecurrenceIfEmpty(recurrence string) {
	if c.Recurrence == "" {
		c.Recurrence = recurrence
	}
}
func (c *Cron) updateTimeZoneIfNotEqual(timezone string) error {
	if !strings.EqualFold(c.Location.String(), timezone) {
		location, err := time.LoadLocation(timezone)
		if err != nil {
			slog.Error("received timezone from the config", "former timezone", c.Location,
				"replaced with", timezone, "error", err, "status", "failed",
			)
			return fmt.Errorf("not possible to load the location. err %v", err)
		}
		slog.Info("received timezone from the config", "former timezone", c.Location,
			"replaced with", timezone, "status", "updated",
		)
		c.Location = location
	}
	return nil
}
