package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/adalbertjnr/downscaler/shared"
)

func (v *Scheduler) ignoredNamespacesCleanupValidation(in map[string]struct{}) {
	if len(v.IgnoredNamespaces) > 0 {
		v.IgnoredNamespaces = nil
		v.IgnoredNamespaces = in
		return
	}
	v.IgnoredNamespaces = in
}

func (c *Scheduler) validateSchedulerNamespaces(ctx context.Context, cronTaskNamespaces []string) bool {
	k8sNamespaces := c.Kubernetes.GetNamespaces(ctx)

	k8sNamespaceSet := make(map[string]struct{})
	for _, k8sNs := range k8sNamespaces {
		k8sNamespaceSet[k8sNs] = struct{}{}
	}

	for _, cronNamespace := range cronTaskNamespaces {
		if cronNamespace == shared.Unspecified {
			continue
		}
		if _, exists := k8sNamespaceSet[cronNamespace]; !exists {
			slog.Error("namespace validation",
				"namespace", cronNamespace,
				"error", ErrNamespaceFromConfigDoNotExists,
				"next retry", "1 minute",
			)
			time.Sleep(time.Minute * 1)
			return false
		}
	}
	return true
}

func validateIfShoudRunUpscalingOrWait(now, targetTimeToUpscale, targetTimeToDownscale time.Time) bool {
	return now.After(targetTimeToDownscale) || now.Before(targetTimeToUpscale)
}

func validateIfShouldRunDownscalingOrWait(now time.Time, currentReplicasState shared.TaskControl, targetTimeToDownscale, targetTimeToUpscale time.Time) bool {
	if currentReplicasState == shared.DeploymentsWithDownscaledState {
		return false
	}
	firstCondition := now.After(targetTimeToUpscale) && currentReplicasState != shared.DeploymentsWithUpscaledState && now.Before(targetTimeToDownscale)
	secondCondition := now.Before(targetTimeToDownscale) && currentReplicasState != shared.DeploymentsWithDownscaledState
	return firstCondition || secondCondition
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

func (c *Scheduler) Validate(downscalerData *shared.DownscalerPolicy) []string {
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
	if err := validateCondition(
		expressionBlock.Operator == "exclude", ErrNotValidExpressionKey,
	); err != "" {
		errors = append(errors, err)
	}

	return errors
}
