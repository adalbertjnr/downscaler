package cron

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

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

func separatedScheduledNamespaces(rules DownscalerRules) map[string]struct{} {
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

func toStringWithFormat(from, now time.Time) (hourString, minuteString string) {
	hourString = fmt.Sprintf("%02d:%02d", from.Hour(), from.Minute())
	minuteString = fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	return hourString, minuteString
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

func (c *Cron) now() time.Time {
	return time.Now().In(c.Location)
}

func logWaitRecurrenceDaysWithSleep(now time.Weekday) {
	slog.Info("time", "today is", now.String(), "recurrence days range", "false",
		"action", "waiting", "next try", "1 minute",
	)
	time.Sleep(time.Minute * 1)
}

func logWaitBeforeDownscalingWithSleep(now time.Time, targetTimeToDownscale time.Time, namespaces []string) {
	ut, nw := toStringWithFormat(targetTimeToDownscale, now)
	slog.Info("task", "current time", nw, "provided crontime", ut,
		"namespace(s)", namespaces, "status", "before downscaling", "next retry", "1 minute",
	)
	time.Sleep(time.Minute * 1)
}

func logWaitAfterDownscalingWithSleep(now time.Time, targetTimeToUpscale time.Time, namespaces []string) {
	fr, nw := toStringWithFormat(targetTimeToUpscale, now)
	slog.Info("task", "current time", nw, "provided crontime", fr,
		"namespace(s)", namespaces, "status", "after downscaling", "next retry", "1 minute",
	)
	time.Sleep(time.Minute * 1)
}
