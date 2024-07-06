package scheduler

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
	"github.com/adalbertjnr/downscaler/kas"
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

type SchedulerTask struct {
	Rules
}

type Scheduler struct {
	Kubernetes        kas.Kubernetes
	Location          *time.Location
	Tasks             []SchedulerTask
	Recurrence        string
	IgnoredNamespaces map[string]struct{}
	taskRoutines      map[string]chan struct{}
	taskch            chan []SchedulerTask
	stopch            chan struct{}
	input             *input.FromArgs
	mu                sync.Mutex
	ctx               context.Context
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		taskch:       make(chan []SchedulerTask),
		stopch:       make(chan struct{}),
		taskRoutines: make(map[string]chan struct{}),
		mu:           sync.Mutex{},
		ctx:          context.Background(),
	}
}

func (c *Scheduler) AddKubeApiSvc(client kas.Kubernetes) *Scheduler {
	c.Kubernetes = client
	return c
}

func (c *Scheduler) AddInput(input *input.FromArgs) *Scheduler {
	c.input = input
	return c
}

func (c *Scheduler) AddSchedulerDetails(downscalerData *shared.DownscalerPolicy) {
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

	c.parseSchedulerConfig(
		recurrence,
		shared.DownscalerExpression{MatchExpressions: expression},
		shared.DownscalerRules{Rules: rules},
	)

}

func (c *Scheduler) parseSchedulerConfig(
	recurrence string,
	expression shared.DownscalerExpression,
	rules shared.DownscalerRules,
) {

	if !stillSameRecurrenceTime(recurrence, c.Recurrence) {
		c.Recurrence = recurrence
	}

	if expression.WithExclude() {
		expressionValues := expression.MatchExpressions.Values
		excludedNamespaces := append([]string{}, expressionValues...)
		ignoredNamespaces := expression.ShowIgnoredNamespaces(excludedNamespaces)

		c.ignoredNamespacesCleanupValidation(ignoredNamespaces)
	}

	if rules.Available() {
		tasks := make([]SchedulerTask, len(rules.Rules))

		scheduledNamespaces := separatedScheduledNamespaces(rules)
		for i, crit := range rules.Rules {
			tasks[i] = SchedulerTask{
				Rules: Rules{
					Namespaces:          crit.Namespaces,
					WithCron:            crit.WithCron,
					Recurrence:          c.Recurrence,
					ScheduledNamespaces: scheduledNamespaces,
				},
			}
		}
		c.killCurrentSchedulerRoutines()
		c.taskch <- tasks
	}
}

func (c *Scheduler) StartScheduler() {
	c.runSchedulerLoop()
}

func (c *Scheduler) runSchedulerLoop() {
	for {
		if tasks, open := <-c.taskch; open {
			c.updateTasks(tasks)
		}
		time.Sleep(time.Second * 5)
		continue
	}
}

func (c *Scheduler) updateTasks(tasks []SchedulerTask) {
	newTaskMap := make(map[string]SchedulerTask)

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

func (c *Scheduler) runTasks(task SchedulerTask, stopch chan struct{}) {
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

	unspecified := shared.NotUsableNamespacesDuringScheduling{
		IgnoredNamespaces:   c.IgnoredNamespaces,
		ScheduledNamespaces: task.ScheduledNamespaces,
	}

	if present := unspecified.Validate(namespaces); present {
		clusterNamespaces := c.Kubernetes.GetNamespaces(c.ctx)
		namespaces = unspecified.ReplaceSpecialFlagWithNamespaces(clusterNamespaces, namespaces)
	}

	for {
		select {
		case <-stopch:
			return
		default:
			targetTimeToUpscale, targetTimeToDownscale := extractUpscalingAndDownscalingTime(task.WithCron, c.Location)
			now := c.now()

			if !c.isRecurrenceDay(now.Weekday(), recurrenceDays) {
				logWaitRecurrenceDaysWithSleep(now.Weekday())
				continue
			}

			if valid := c.validateSchedulerNamespaces(c.ctx, namespaces); !valid {
				continue
			}

			currentReplicasState, err := c.inspectReplicasStateByNamespace(c.ctx, namespaces)
			if err != nil && currentReplicasState == shared.InspectError {
				slog.Error("inspect replicas by namespace error", "err", err)
				continue
			}

			if validateIfShouldRunDownscalingOrWait(now, currentReplicasState, targetTimeToDownscale, targetTimeToUpscale) {
				logWaitBeforeDownscalingWithSleep(now, targetTimeToDownscale, namespaces)
				continue
			}

			if currentReplicasState == shared.DeploymentsWithUpscaledState || currentReplicasState == shared.AppStartupWithNoDataWrite || currentReplicasState == shared.UpscalingDeactivated {
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

func (c *Scheduler) killCurrentSchedulerRoutines() {
	if len(c.taskRoutines) > 0 {
		for key, stopch := range c.taskRoutines {
			delete(c.taskRoutines, key)
			close(stopch)
		}
	}
}

func (c *Scheduler) MustAddTimezoneLocation(timeZone string) *Scheduler {
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		panic(err)
	}
	slog.Info("received timezone from the config", "timezone", timeZone)
	c.Location = location
	return c
}

func (c *Scheduler) isRecurrenceDay(day time.Weekday, recurrenceDays []time.Weekday) bool {
	for _, recurrenceDay := range recurrenceDays {
		if day == recurrenceDay {
			return true
		}
	}
	return false
}

func (c *Scheduler) updateRecurrenceIfEmpty(recurrence string) {
	if c.Recurrence == "" {
		c.Recurrence = recurrence
	}
}

func (c *Scheduler) inspectReplicasStateByNamespace(ctx context.Context, namespaces []string) (shared.TaskControl, error) {
	if c.input.RunUpscaling {
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

		if !namespaceIndexAvailable(namespaces, cm) {
			return shared.AppStartupWithNoDataWrite, nil
		}

		for _, metadata := range namespaceState {
			if metadata.State == nil && metadata.Status == shared.EmptyNamespace {
				continue
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
	}
	return shared.UpscalingDeactivated, nil
}

func (c *Scheduler) checkIfNamespaceExistsInConfigMapBeforeWrite(ctx context.Context, cmCurrentState map[string]shared.Apps, cmData map[string]string) error {
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

func (c *Scheduler) writeCmValueByNamespaceKey(ctx context.Context, cmCurrentState map[string]shared.Apps) error {
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

func (c *Scheduler) writeOldStateDeploymentsReplicas(ctx context.Context, cmCurrentState map[string]shared.Apps) error {
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

func (c *Scheduler) updateTimeZoneIfNotEqual(timezone string) error {
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

func (c *Scheduler) handleDownscaling(task SchedulerTask, namespaces []string) {
	notUsableNamespaces := shared.NotUsableNamespacesDuringScheduling{
		IgnoredNamespaces:   c.IgnoredNamespaces,
		ScheduledNamespaces: task.ScheduledNamespaces,
	}

	toCmCurrentState := c.Kubernetes.StartDownscaling(c.ctx, namespaces, notUsableNamespaces)
	err := c.writeOldStateDeploymentsReplicas(c.ctx, toCmCurrentState)
	if err != nil {
		slog.Error("error writing state after downscaling", "err", err)
	}
}

func (c *Scheduler) handleUpscaling(task SchedulerTask, stopch chan struct{}, namespaces []string) shared.TaskControl {
	for {
		select {
		case <-stopch:
			return shared.KillCurrentRoutine
		default:
			now := c.now()
			targetTimeToUpscale, targetTimeToDownscale := extractUpscalingAndDownscalingTime(task.WithCron, c.Location)

			response, err := c.inspectReplicasStateByNamespace(c.ctx, namespaces)
			if err != nil && response == shared.InspectError {
				slog.Error("inspect replicas by namespace error", "err", err)
				continue
			}

			if validateIfShoudRunUpscalingOrWait(now, targetTimeToUpscale, targetTimeToDownscale) {
				logWaitAfterDownscalingWithSleep(now, targetTimeToUpscale, namespaces)
				continue
			}

			if response == shared.DeploymentsWithDownscaledState {
				cmAppsSlice := c.Kubernetes.StartUpscaling(c.ctx, task.ScheduledNamespaces, namespaces, c.input.ConfigMapName, c.input.ConfigMapNamespace)
				for _, cmApps := range cmAppsSlice {
					if err := c.writeCmValueByNamespaceKey(c.ctx, cmApps); err != nil {
						slog.Error("error writing state after upscaling", "err", err)
					}
				}
				return shared.RestartRoutine
			}
		}
	}
}
