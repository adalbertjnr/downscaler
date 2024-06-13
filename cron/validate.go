package cron

import (
	"context"
	"log/slog"
	"strings"

	"github.com/adalbertjnr/downscaler/shared"
)

func (v *Cron) ignoredNamespacesCleanupValidation(in map[string]struct{}) {
	if len(v.IgnoredNamespaces) > 0 {
		v.IgnoredNamespaces = nil
		v.IgnoredNamespaces = in
		return
	}
	v.IgnoredNamespaces = in
}

func (c *Cron) validateCronNamespaces(ctx context.Context, cronTaskNamespaces []string) bool {
	k8sNamespaces := c.Kubernetes.GetNamespaces(ctx)

	k8sNamespaceSet := make(map[string]struct{})
	for _, k8sNs := range k8sNamespaces {
		k8sNamespaceSet[k8sNs] = struct{}{}
	}

	for _, cronNamespace := range cronTaskNamespaces {
		if cronNamespace == shared.AnyOther {
			continue
		}
		if _, exists := k8sNamespaceSet[cronNamespace]; !exists {
			slog.Error("namespace validation",
				"namespace", cronNamespace,
				"error", ErrNamespaceFromConfigDoNotExists,
				"next retry", "1 minute",
			)
			return false
		}
	}
	return true
}

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
