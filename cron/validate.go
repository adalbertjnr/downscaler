package cron

import (
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
)

func stillSameRecurrenceTime(currentRecurrence, newRecurrence string) bool {
	return currentRecurrence != "" && strings.EqualFold(currentRecurrence, newRecurrence)
}

func validateCondition(condition bool, errorsMsg string) string {
	if !condition {
		return errorsMsg
	}
	return ""
}

func (c *Cron) Validate(downscalerData *shared.DownscalerPolicy) []string {
	errors := make([]string, 0)
	timeBlock := downscalerData.Spec.ExecutionOpts.Time
	expressionBlock := downscalerData.Spec.ExecutionOpts.Time.Downscaler.DownscalerSelectorTerms.MatchExpressions

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
