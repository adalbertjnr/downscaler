package cron

import (
	"log/slog"
	"strings"
	"time"

	"github.com/adalbertjnr/downscaler/shared"
)

const ExpectedTimeParts = 2

type Downscalerfn func()

type Criteria struct {
	Namespace []string
	WithCron  string
}

type CronTask struct {
	Criteria
	Downscalerfn
}

type Cron struct {
	Hour     time.Time
	Minute   time.Time
	Location *time.Location
	C        Criteria
	Tasks    []CronTask
}

func NewCron() *Cron {
	return &Cron{}
}

func (c *Cron) AddYamlPolicy(downscalerData *shared.DownscalerPolicy) {
	if errors := c.Validate(downscalerData); len(errors) > 0 {
		for _, err := range errors {
			slog.Error("crontime validator", "error", err)
		}
		return
	}

	c.ParseRecurrenceTime()
}

func (c *Cron) ParseRecurrenceTime() {
	slog.Info("crontime trigger received")
}

func (c *Cron) StartCron() {
}

func validateCondition(condition bool, errorsMsg string) string {
	if !condition {
		return errorsMsg
	}
	return ""
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

func (c *Cron) Validate(downscalerData *shared.DownscalerPolicy) []string {
	errors := make([]string, 0)
	timeBlock := downscalerData.Spec.ExecutionOpts.Time
	expressionBlock := downscalerData.Spec.ExecutionOpts.Downscaler.DownscalerSelectorTerms.MatchExpressions

	if err := validateCondition(timeBlock.DefaultUptime != "", ErrDefaultUpTimeNotFound); err != "" {
		errors = append(errors, err)
	}
	if err := validateCondition(timeBlock.TimeZone != "", ErrTimeZoneNotFound); err != "" {
		errors = append(errors, err)
	}
	if err := validateCondition(timeBlock.Recurrence != "", ErrRecurrenceTimeNotFound); err != "" {
		errors = append(errors, err)
	}
	if err := validateCondition(expressionBlock.Key == "namespace", ErrNotValidExpressionKey); err != "" {
		errors = append(errors, err)
	}
	if err := validateCondition(len(expressionBlock.Values) > 0, ErrExpressionsValuesAreEmpty); err != "" {
		errors = append(errors, err)
	}
	if err := validateCondition(
		expressionBlock.Operator == "exclude" || expressionBlock.Operator == "notExclude",
		ErrNotValidExpressionKey,
	); err != "" {
		errors = append(errors, err)
	}

	return errors
}

func getCronFromAndUntil(time string) (from, until string) {
	parts := strings.SplitN(time, "-", ExpectedTimeParts)
	if len(parts) != ExpectedTimeParts {
		slog.Error("error parsing cron time", "time received", time)
		return
	}
	return from, until
}
