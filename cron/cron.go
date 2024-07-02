package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adalbertjnr/downscaler/helpers"
	"github.com/adalbertjnr/downscaler/input"
	k8s "github.com/adalbertjnr/downscaler/k8sutil"
	"github.com/adalbertjnr/downscaler/shared"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
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
	Recurrence        string
	IgnoredNamespaces map[string]struct{}
	taskRoutines      map[string]chan struct{}
	taskch            chan []CronTask
	stopch            chan struct{}
	input             *input.FromArgs
	mu                sync.Mutex
	ctx               context.Context
}

func NewCron() *Cron {
	return &Cron{
		taskch:       make(chan []CronTask),
		stopch:       make(chan struct{}),
		taskRoutines: make(map[string]chan struct{}),
		mu:           sync.Mutex{},
		ctx:          context.Background(),
	}
}

func (c *Cron) AddKubeApiSvc(client k8s.Kubernetes) *Cron {
	c.Kubernetes = client
	return c
}

func (c *Cron) AddInput(input *input.FromArgs) *Cron {
	c.input = input
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
		}
	}
}

func (c *Cron) runTasks(task CronTask, stopch chan struct{}) {
	slog.Info("task", "provided namespace(s)", task.Namespaces, "period time", task.WithCron,
		"recurrence", task.Recurrence, "status", "initializing",
	)

	defer func() {
		slog.Info("task", "provided namespace(s)", task.Namespaces, "period time", task.WithCron,
			"recurrence", task.Recurrence, "status", "terminated",
		)
	}()

	var (
		namespaces     = task.Namespaces
		recurrence     = task.Recurrence
		recurrenceDays = parseRecurrence(recurrence)
	)

	for {
		select {
		case <-stopch:
			return
		default:
			_, targetTimeToDownscale := fromUntil(task.WithCron, c.Location)
			now := c.now()

			if !c.isRecurrenceDay(now.Weekday(), recurrenceDays) {
				logWaitRecurrenceDaysWithSleep(now.Weekday())
				continue
			}

			if valid := c.validateCronNamespaces(c.ctx, namespaces); !valid {
				continue
			}

			if now.Before(targetTimeToDownscale) {
				logWaitBeforeDownscalingWithSleep(now, targetTimeToDownscale, namespaces)
				continue
			}

			currentReplicasState, err := c.inspectReplicasStateByNamespace(c.ctx, namespaces, task.ScheduledNamespaces)

			if err != nil && currentReplicasState == shared.InspectError {
				continue
			}

			if currentReplicasState == shared.DeploymentsWithUpscaledState || currentReplicasState == shared.AppStartupWithNoDataWrite {
				c.handleDownscaling(task, namespaces)

				next := c.handleUpscaling(task, stopch, namespaces)
				if next == shared.KillCurrentRoutine {
					return
				}

				if next == shared.RestartRoutine {
					continue
				}
			}

			if currentReplicasState == shared.DeploymentsWithDownscaledState {
				next := c.handleUpscaling(task, stopch, namespaces)
				if next == shared.KillCurrentRoutine {
					return
				}
			}
		}
	}

}

func (c *Cron) killCurrentCronRoutines() {
	if len(c.taskRoutines) > 0 {
		for key, stopch := range c.taskRoutines {
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

func (c *Cron) inspectReplicasStateByNamespace(ctx context.Context, namespaces []string, scheduledNamespaces map[string]struct{}) (shared.TaskControl, error) {
	if c.input.RunUpscaling {
		c.mu.Lock()
		namespaceState := make(map[string]shared.Apps)
		cm := c.Kubernetes.ListConfigMap(ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace)

		err := helpers.UnmarshalDataPolicy(cm, namespaceState)
		if err != nil {
			slog.Error("error unmarshaling data time policy", "error", err)
			return -1, err
		}

		var (
			replicaCountSum int = 0
			notEmptyIndex   int = 0
		)

		if cm.Data == nil {
			return shared.AppStartupWithNoDataWrite, nil
		}

		for namespace, metadata := range namespaceState {
			if metadata.State == nil && metadata.Status == shared.EmptyNamespace {
				continue
			}

			if cm.Data[namespace] == "" {
				return shared.AppStartupWithNoDataWrite, nil
			}

			sum, count, err := parseReplicaState(metadata.State)
			if err != nil {
				slog.Error("conversion error", "err", err)
				return shared.InspectError, err
			}
			replicaCountSum += sum
			notEmptyIndex += count
		}

		if replicaCountSum/notEmptyIndex == int(shared.DeploymentsWithDownscaledState) {
			return shared.DeploymentsWithDownscaledState, nil
		}

		if replicaCountSum/notEmptyIndex == int(shared.DeploymentsWithUpscaledState) {
			return shared.DeploymentsWithUpscaledState, nil
		}
		c.mu.Unlock()
	}
	return shared.UpscalingDeactivated, nil
}

func (c *Cron) checkIfNamespaceExistsInConfigMapBeforeWrite(ctx context.Context, cmCurrentState map[string]shared.Apps, cmData map[string]string) error {
	for namespace := range cmCurrentState {
		namespaceYamlKey := getNamespaceWithYamlExt(namespace)
		if _, exists := cmData[namespaceYamlKey]; !exists {
			patchWithKey, err := createConfigMapKeyForPatching(namespaceYamlKey)
			if err != nil {
				slog.Error("error creating configmap key for patching", "error", err)
				return err
			}
			c.Kubernetes.PatchConfigMap(ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace, patchWithKey)
		}
	}
	return nil
}

func (c *Cron) writeCmValueByNamespaceKey(ctx context.Context, cmCurrentState map[string]shared.Apps) error {
	for namespace := range cmCurrentState {
		var (
			namespaceKey   = getNamespaceWithYamlExt(namespace)
			metadataFromCm = cmCurrentState[namespace]
		)

		patchWith := shared.Apps{
			Status: metadataFromCm.Status,
			Group:  metadataFromCm.Group,
			State:  extractSegments(metadataFromCm),
		}

		yamlData, err := yaml.Marshal(patchWith)
		if err != nil {
			return err
		}
		patch := &corev1.ConfigMap{
			Data: map[string]string{
				namespaceKey: string(yamlData),
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return err
		}
		c.Kubernetes.PatchConfigMap(ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace, patchBytes)
	}
	return nil
}

func (c *Cron) writeOldStateDeploymentsReplicas(ctx context.Context, cmCurrentState map[string]shared.Apps) error {
	if c.input.RunUpscaling {
		currentCm := c.Kubernetes.ListConfigMap(ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace)
		err := c.checkIfNamespaceExistsInConfigMapBeforeWrite(ctx, cmCurrentState, currentCm.Data)
		if err != nil {
			return err
		}

		if err := c.writeCmValueByNamespaceKey(ctx, cmCurrentState); err != nil {
			return err
		}
	}
	return nil
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

func (c *Cron) handleDownscaling(task CronTask, namespaces []string) {
	notUsableNamespaces := shared.NotUsableNamespacesDuringScheduling{
		IgnoredNamespaces:   c.IgnoredNamespaces,
		ScheduledNamespaces: task.ScheduledNamespaces,
	}

	toCmCurrentState := c.Kubernetes.StartDownscaling(c.ctx, namespaces, notUsableNamespaces)
	err := c.writeOldStateDeploymentsReplicas(c.ctx, toCmCurrentState)
	if err != nil {
		slog.Error("error writing the old state replicas", "error", err)
	}
}

func (c *Cron) handleUpscaling(task CronTask, stopch chan struct{}, namespaces []string) shared.TaskControl {
	for {
		select {
		case <-stopch:
			return shared.KillCurrentRoutine
		default:
			now := c.now()
			targetTimeToUpscale, targetTimeToDownscale := fromUntil(task.WithCron, c.Location)

			response, err := c.inspectReplicasStateByNamespace(c.ctx, namespaces, task.ScheduledNamespaces)
			if err != nil && response == shared.InspectError {
				continue
			}

			if now.Before(targetTimeToUpscale) || (now.After(targetTimeToDownscale) || response == shared.DeploymentsWithUpscaledState) {
				logWaitAfterDownscalingWithSleep(now, targetTimeToUpscale, namespaces)
				continue
			}

			//replicas available to upscaling which means
			// all deployments replicas are equals 0
			if response == shared.DeploymentsWithDownscaledState {
				// c.Kubernetes.StartUpscaling(c.ctx, c.input.ConfigMapName, c.input.ConfigMapNamespace)
				//handle upscaling
				//writeReplicas
				//change replicas to > 0
				//overriding current state to DeploymentsWithUpscaledState
				return shared.RestartRoutine
			}

			//replicas are not available to upscaling which means
			// all deployments replicas are > 0
			return shared.RestartRoutine
		}
	}
}
