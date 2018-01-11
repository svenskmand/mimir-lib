package tracked

import (
	"context"
	"fmt"
	"reflect"
	"time"

	pb_job "code.uber.internal/infra/peloton/.gen/peloton/api/job"
	pb_task "code.uber.internal/infra/peloton/.gen/peloton/api/task"
	"code.uber.internal/infra/peloton/util"

	log "github.com/sirupsen/logrus"
)

// taskStatesAfterStart is the set of Peloton task states which
// indicate a task is being or has already been started.
var taskStatesAfterStart = []pb_task.TaskState{
	pb_task.TaskState_STARTING,
	pb_task.TaskState_RUNNING,
	pb_task.TaskState_SUCCEEDED,
	pb_task.TaskState_FAILED,
	pb_task.TaskState_LOST,
	pb_task.TaskState_PREEMPTING,
	pb_task.TaskState_KILLING,
	pb_task.TaskState_KILLED,
}

var taskStatesScheduled = []pb_task.TaskState{
	pb_task.TaskState_RUNNING,
	pb_task.TaskState_PENDING,
	pb_task.TaskState_LAUNCHED,
	pb_task.TaskState_READY,
	pb_task.TaskState_PLACING,
	pb_task.TaskState_PLACED,
	pb_task.TaskState_LAUNCHING,
	pb_task.TaskState_STARTING,
	pb_task.TaskState_PREEMPTING,
	pb_task.TaskState_KILLING,
}

// formatTime converts a Unix timestamp to a string format of the
// given layout in UTC. See https://golang.org/pkg/time/ for possible
// time layout in golang. For example, it will return RFC3339 format
// string like 2017-01-02T11:00:00.123456789Z if the layout is
// time.RFC3339Nano
func formatTime(timestamp float64, layout string) string {
	seconds := int64(timestamp)
	nanoSec := int64((timestamp - float64(seconds)) *
		float64(time.Second/time.Nanosecond))
	return time.Unix(seconds, nanoSec).UTC().Format(layout)
}

func (j *job) startInstances(ctx context.Context, runtime *pb_job.RuntimeInfo, maxRunningInstances uint32) error {
	if runtime.GetGoalState() == pb_job.JobState_KILLED {
		return nil
	}

	stateCounts := runtime.GetTaskStats()

	currentScheduledInstances := uint32(0)
	for _, state := range taskStatesScheduled {
		currentScheduledInstances += stateCounts[state.String()]
	}

	if currentScheduledInstances >= maxRunningInstances {
		log.WithField("current_scheduled_tasks", currentScheduledInstances).
			WithField("job_id", j.ID().GetValue()).
			Debug("no instances to start")
	}
	tasksToStart := maxRunningInstances - currentScheduledInstances

	initializedTasks, err := j.m.taskStore.GetTaskIDsForJobAndState(ctx, j.ID(), pb_task.TaskState_INITIALIZED.String())
	if err != nil {
		log.WithError(err).
			WithField("job_id", j.ID().GetValue()).
			Error("failed to fetch initialized task list")
		return err
	}

	for _, instID := range initializedTasks {
		if tasksToStart == 0 {
			break
		}

		// MV view may run behind. So, make sure that task state is indeed INITIALIZED.
		taskRuntime, err := j.m.taskStore.GetTaskRuntime(ctx, j.ID(), instID)
		if err != nil {
			log.WithError(err).
				WithField("job_id", j.ID().GetValue()).
				WithField("instance_id", instID).
				Error("failed to fetch task runtimeme")
			continue
		}

		if taskRuntime.GetState() != pb_task.TaskState_INITIALIZED {
			// Task wrongly set to INITIALIZED, ignore.
			tasksToStart--
			continue
		}

		t := j.GetTask(instID)
		if t.IsScheduled() {
			continue
		}
		j.m.taskScheduler.schedule(t.(*task), time.Now())
		tasksToStart--
	}

	// Keeping the commented code when we have write through cache, then we
	// can read from cache instead of DB.
	/*j.RLock()
	defer j.RUnlock()

	for _, task := range j.initializedTasks {
		if tasksToStart == 0 {
			// TBD remove this log after testing
			log.WithField("job_id", j.id.GetValue()).
				WithField("started_tasks", (maxRunningInstances - currentScheduledInstances)).
				Info("scheduled tasks")
			break
		}

		if task.IsScheduled() {
			continue
		}

		j.m.taskScheduler.schedule(task, time.Now())
		tasksToStart--
	}*/
	return nil
}

// JobRuntimeUpdater updates the job runtime.
// When the jobmgr leader fails over, the tracked.manager runs syncFromDB which enqueues all recovered jobs
// into goal state, which will then run the job runtime updater and update the out-of-date runtime info.
func (j *job) JobRuntimeUpdater(ctx context.Context) (bool, error) {
	log.WithField("job_id", j.ID().GetValue()).
		Info("running job runtime update")

	jobRuntime, err := j.m.jobStore.GetJobRuntime(ctx, j.ID())
	if err != nil {
		log.WithError(err).
			WithField("job_id", j.ID().GetValue()).
			Error("failed to get job runtime in runtime updater")
		j.m.mtx.jobMetrics.JobRuntimeUpdateFailed.Inc(1)
		return true, err
	}

	var jConfig *JobConfig
	jConfig, err = j.GetConfig()
	if err != nil {
		jobConfig, err := j.m.jobStore.GetJobConfig(ctx, j.ID())
		if err != nil {
			log.WithError(err).
				WithField("job_id", j.ID().GetValue()).
				Error("failed to get job config in runtime updater")
			j.m.mtx.jobMetrics.JobRuntimeUpdateFailed.Inc(1)
			return true, err
		}
		j.updateRuntime(&pb_job.JobInfo{
			Config:  jobConfig,
			Runtime: jobRuntime,
		})
		jConfig, _ = j.GetConfig()
	}
	instances := jConfig.instanceCount

	// Keeping the commented code when we have write through cache, then we
	// can read from cache instead of DB.
	/*stateCounts := make(map[string]uint32)
	taskMap := j.GetTasks()
	j.clearInitializedTaskMap()

	for _, task := range taskMap {
		runtime := task.GetRunTime()
		retry := 0
		for retry < 1000 {
			if runtime != nil {
				break
			}
			time.Sleep(1 * time.Millisecond)
			log.WithField("job_id", j.ID()).
			    WithField("instance_id", task.ID()).
			    Info("reloading the task runtime within job runtime updater")
			task.reloadRuntime(ctx)
			retry++
		}
		if runtime == nil {
			return true, fmt.Errorf("cannot fetch task runtime")
		}

		stateCounts[runtime.GetState().String()]++

		if runtime.GetState() == pb_task.TaskState_INITIALIZED && runtime.GetGoalState() != pb_task.TaskState_KILLED {
			j.addTaskToInitializedTaskMap(task.(*task))
		}
	}*/

	stateCounts, err := j.m.taskStore.GetTaskStateSummaryForJob(ctx, j.ID())
	if err != nil {
		log.WithError(err).
			WithField("job_id", j.ID().GetValue()).
			Error("failed to fetch task state summary")
		return true, err
	}

	if jobRuntime.GetTaskStats() != nil && reflect.DeepEqual(stateCounts, jobRuntime.GetTaskStats()) {
		log.WithField("job_id", j.ID().GetValue()).
			WithField("task_stats", stateCounts).
			Debug("Task stats did not change, return")
		return false, nil
	}

	getFirstTaskUpdateTime := j.getFirstTaskUpdateTime()
	if getFirstTaskUpdateTime != 0 && jobRuntime.StartTime == "" {
		count := uint32(0)
		for _, state := range taskStatesAfterStart {
			count += stateCounts[state.String()]
		}

		if count > 0 {
			jobRuntime.StartTime = formatTime(getFirstTaskUpdateTime, time.RFC3339Nano)
		}
	}

	completionTime := ""
	lastTaskUpdateTime := j.getLastTaskUpdateTime()
	if lastTaskUpdateTime != 0 {
		completionTime = formatTime(lastTaskUpdateTime, time.RFC3339Nano)
	}

	totalInstanceCount := uint32(0)
	for _, state := range pb_task.TaskState_name {
		totalInstanceCount += stateCounts[state]
	}

	if totalInstanceCount != instances {
		// Either MV view has not caught up or all instances have not been created
		if j.GetTasksNum() != instances {
			// all instances have not been created, trigger recovery
			jobRuntime.State = pb_job.JobState_INITIALIZED
		} else {
			// MV has not caught up, wait for it to catch up before doing anything
			return true, fmt.Errorf("dbs are not in sync")
		}
	}

	var jobState pb_job.JobState
	if jobRuntime.State == pb_job.JobState_INITIALIZED && j.GetTasksNum() != instances {
		// do not do any thing as all tasks have not been created yet
		jobState = pb_job.JobState_INITIALIZED
	} else if stateCounts[pb_task.TaskState_SUCCEEDED.String()] == instances {
		jobState = pb_job.JobState_SUCCEEDED
		jobRuntime.CompletionTime = completionTime
		j.m.mtx.jobMetrics.JobSucceeded.Inc(1)
	} else if stateCounts[pb_task.TaskState_SUCCEEDED.String()]+
		stateCounts[pb_task.TaskState_FAILED.String()] == instances {
		jobState = pb_job.JobState_FAILED
		jobRuntime.CompletionTime = completionTime
		j.m.mtx.jobMetrics.JobFailed.Inc(1)
	} else if stateCounts[pb_task.TaskState_KILLED.String()] > 0 &&
		(stateCounts[pb_task.TaskState_SUCCEEDED.String()]+
			stateCounts[pb_task.TaskState_FAILED.String()]+
			stateCounts[pb_task.TaskState_KILLED.String()] == instances) {
		jobState = pb_job.JobState_KILLED
		jobRuntime.CompletionTime = completionTime
		j.m.mtx.jobMetrics.JobKilled.Inc(1)
	} else if stateCounts[pb_task.TaskState_RUNNING.String()] > 0 {
		jobState = pb_job.JobState_RUNNING
	} else {
		jobState = pb_job.JobState_PENDING
	}

	jobRuntime.State = jobState
	jobRuntime.TaskStats = stateCounts

	// Update the job runtime
	err = j.m.jobStore.UpdateJobRuntime(ctx, j.ID(), jobRuntime)
	if err != nil {
		log.WithError(err).
			WithField("job_id", j.ID().GetValue()).
			Error("failed to update jobRuntime in runtime updater")
		j.m.mtx.jobMetrics.JobRuntimeUpdateFailed.Inc(1)
		return true, err
	}

	if jConfig.sla.GetMaximumRunningInstances() > 0 {
		err = j.startInstances(ctx, jobRuntime, jConfig.sla.GetMaximumRunningInstances())
		if err != nil {
			return true, err
		}
	}

	if util.IsPelotonJobStateTerminal(jobRuntime.GetState()) {
		j.m.ScheduleJob(j, time.Now())
	}

	log.WithField("job_id", j.ID().GetValue()).
		WithField("updated_state", jobRuntime.State.String()).
		Info("job runtime updater completed")

	j.m.mtx.jobMetrics.JobRuntimeUpdated.Inc(1)
	if jobRuntime.State == pb_job.JobState_INITIALIZED {
		// This should be hit for only old jobs created with a code version with no job goal state
		return true, fmt.Errorf("trigger job recovery")
	}
	return false, nil
}