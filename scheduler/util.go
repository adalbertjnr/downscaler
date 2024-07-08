package scheduler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/adalbertjnr/downscaler/shared"
	corev1 "k8s.io/api/core/v1"
)

func namespaceIndexAvailable(namespaces []string, cm *corev1.ConfigMap) bool {
	if cm.Data == nil {
		return false
	}

	for _, taskNamespace := range namespaces {
		if cm.Data[taskNamespace+".yaml"] == "" {
			return false
		}
	}

	return true
}

func extractSegments(apps shared.Apps) []string {
	cmData := make([]string, len(apps.State))
	for i, value := range apps.State {
		parts := strings.Split(value, ",")
		cmData[i] = fmt.Sprintf("%s,%s,%s", parts[0], parts[1], parts[2])
	}

	return cmData
}

func getNamespaceWithYamlExt(namespace string) string {
	return namespace + ".yaml"
}

func createConfigMapKeyForPatching(key string) ([]byte, error) {
	data := &corev1.ConfigMap{
		Data: map[string]string{
			key: "",
		},
	}

	patch, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return patch, nil
}

func parseReplicaState(deploymentWithState []string) (int, int, error) {
	replicaCountSum := 0
	notEmptyIndex := 0
	for _, appDeploymentWithState := range deploymentWithState {
		replicasStr := strings.TrimSpace(strings.Split(appDeploymentWithState, ",")[2])
		replicasInt, err := strconv.Atoi(replicasStr)
		if err != nil {
			slog.Error("conversion error", "err", err)
			return 0, 0, err
		}
		replicaCountSum += replicasInt
		notEmptyIndex++
	}
	return replicaCountSum, notEmptyIndex, nil
}

func separatedScheduledNamespaces(rules shared.DownscalerRules) map[string]struct{} {
	scheduledNamespaces := make([]string, 0)
	for _, criteria := range rules.Rules {
		scheduledNamespaces = append(scheduledNamespaces, criteria.Namespaces...)
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

func shouldConvertTimeFormat(timeFromRules string) bool {
	return strings.Contains(timeFromRules, "PM")
}

func convertTimeFormat(timeFromRules, timeFormat string) (upscalingTime, downscalingTime time.Time, err error) {
	timeParts := strings.SplitN(timeFromRules, "-", shared.ExpectedTimeParts)
	if len(timeParts) != shared.ExpectedTimeParts {
		slog.Error("error parsing cron time", "time received", timeFromRules)
		return upscalingTime, downscalingTime, fmt.Errorf("invalid time format")
	}

	switch timeFormat {
	case shared.Default24TimeFormat:
		upscalingTime, err = time.Parse(shared.TimeFormat, timeParts[0])
		if err != nil {
			slog.Error("error parsing the time from", "time", timeParts[0])
			return
		}
		downscalingTime, err = time.Parse(shared.TimeFormat, timeParts[1])
		if err != nil {
			slog.Error("error parsing the time until", "time", timeParts[1])
			return
		}

	case shared.Default12TimeFormat:
		upscalingTime, err = time.Parse(shared.TimeFormat12Ups, timeParts[0])
		if err != nil {
			return upscalingTime, downscalingTime, err
		}

		downscalingTime, err = time.Parse(shared.TimeFormat12Down, timeParts[1])
		if err != nil {
			return upscalingTime, downscalingTime, err
		}

	default:
		return upscalingTime, downscalingTime, fmt.Errorf("convert time format error - provided time format not found")
	}

	return upscalingTime, downscalingTime, nil
}

func extractUpscalingAndDownscalingTime(timeFromRules string, loc *time.Location) (upscalingTime, downscalingTime time.Time) {
	var err error
	timeParts := strings.SplitN(timeFromRules, "-", shared.ExpectedTimeParts)
	if len(timeParts) != shared.ExpectedTimeParts {
		slog.Error("crontime parsing error", "time received", timeFromRules)
		return
	}

	if shouldConvertTimeFormat(timeParts[1]) {
		upscalingTime, downscalingTime, err = convertTimeFormat(timeFromRules, shared.Default12TimeFormat)
		if err != nil {
			slog.Error("crontime parsing error", "time received", timeFromRules, "err", err)
			return
		}
	} else {
		upscalingTime, downscalingTime, err = convertTimeFormat(timeFromRules, shared.Default24TimeFormat)
		if err != nil {
			slog.Error("crontime parsing error", "time received", timeFromRules, "err", err)
			return
		}
	}

	now := time.Now().In(loc)
	upscalingTime = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		upscalingTime.Hour(),
		upscalingTime.Minute(),
		0,
		0,
		loc,
	)

	downscalingTime = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		downscalingTime.Hour(),
		downscalingTime.Minute(),
		0,
		0,
		loc,
	)

	return upscalingTime, downscalingTime
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

func (c *Scheduler) now() time.Time {
	return time.Now().In(c.Location)
}

func logWaitRecurrenceDaysWithSleep(now time.Weekday) {
	slog.Info("time", "today is", now.String(), "recurrence days range", "false", "action", "waiting", "next try", "1 minute")
	time.Sleep(time.Minute * 1)
}

func logWaitBeforeDownscalingWithSleep(now time.Time, targetTimeToDownscaleFromConfig string, namespaces []string) {
	var (
		nowStringFormatted   = fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
		targetTimeToDowscale = strings.Split(targetTimeToDownscaleFromConfig, "-")
	)

	slog.Info("task", "current time", nowStringFormatted, "provided crontime", targetTimeToDowscale[1], "status", "before downscaling", "next retry", "1 minute", "namespace(s)", namespaces)
	time.Sleep(time.Minute * 1)
}

func logWaitAfterDownscalingWithSleep(now time.Time, targetTimeToUpscaleFromConfig string, namespaces []string) {
	var (
		nowStringFormatted  = fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
		targetTimeToUpscale = strings.Split(targetTimeToUpscaleFromConfig, "-")
	)

	slog.Info("task", "current time", nowStringFormatted, "provided crontime", targetTimeToUpscale[0], "status", "after downscaling", "next retry", "1 minute", "namespace(s)", namespaces)
	time.Sleep(time.Minute * 1)
}
