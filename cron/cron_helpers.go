package cron

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func separatedScheduledNamespaces(criterias DownscalerCriteria) map[string]struct{} {
	scheduledNamespaces := make([]string, 0)
	for _, criteria := range criterias.Criteria {
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
