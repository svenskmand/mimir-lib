package goalstate

import (
	"context"
	"testing"

	pbjob "code.uber.internal/infra/peloton/.gen/peloton/api/job"
	"code.uber.internal/infra/peloton/.gen/peloton/api/peloton"
	pbtask "code.uber.internal/infra/peloton/.gen/peloton/api/task"

	goalstatemocks "code.uber.internal/infra/peloton/common/goalstate/mocks"
	"code.uber.internal/infra/peloton/jobmgr/cached"
	cachedmocks "code.uber.internal/infra/peloton/jobmgr/cached/mocks"
	storemocks "code.uber.internal/infra/peloton/storage/mocks"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally"
)

func TestJobKill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := storemocks.NewMockJobStore(ctrl)
	taskStore := storemocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)

	goalStateDriver := &driver{
		jobEngine:  jobGoalStateEngine,
		taskEngine: taskGoalStateEngine,
		jobStore:   jobStore,
		taskStore:  taskStore,
		jobFactory: jobFactory,
		mtx:        NewMetrics(tally.NoopScope),
		cfg:        &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}

	jobEnt := &jobEntity{
		id:     jobID,
		driver: goalStateDriver,
	}

	instanceCount := uint32(4)
	jobConfig := pbjob.JobConfig{
		OwningTeam:    "team6",
		LdapGroups:    []string{"team1", "team2", "team3"},
		InstanceCount: instanceCount,
		Type:          pbjob.JobType_BATCH,
	}

	jobStore.EXPECT().
		GetJobConfig(gomock.Any(), jobID).
		Return(&jobConfig, nil)

	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[0] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[1] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}
	runtimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_SUCCEEDED,
	}

	newRuntimes := make(map[uint32]*pbtask.RuntimeInfo)
	newRuntimes[0] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_KILLED,
		Message:   "Task stop API request",
		Reason:    "",
	}
	newRuntimes[1] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_KILLED,
		Message:   "Task stop API request",
		Reason:    "",
	}
	newRuntimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_KILLED,
		Message:   "Task stop API request",
		Reason:    "",
	}
	newRuntimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_INITIALIZED,
		GoalState: pbtask.TaskState_KILLED,
		Message:   "Task stop API request",
		Reason:    "",
	}

	jobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_RUNNING,
		GoalState: pbjob.JobState_SUCCEEDED,
	}
	newJobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_KILLING,
		GoalState: pbjob.JobState_SUCCEEDED,
	}

	taskStore.EXPECT().
		GetTaskRuntimesForJobByRange(gomock.Any(), jobID, gomock.Any()).
		Return(runtimes, nil)
	jobFactory.EXPECT().
		GetJob(jobID).
		Return(cachedJob).AnyTimes()
	cachedJob.EXPECT().
		UpdateTasks(gomock.Any(), newRuntimes, cached.UpdateCacheAndDB).
		Return(nil)
	cachedJob.EXPECT().
		GetJobType().Return(pbjob.JobType_BATCH).Times(int(instanceCount))
	taskGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return().Times(int(instanceCount))
	jobGoalStateEngine.EXPECT().
		Enqueue(gomock.Any(), gomock.Any()).
		Return().Times(int(instanceCount))
	jobStore.EXPECT().
		GetJobRuntime(gomock.Any(), jobID).
		Return(jobRuntime, nil)
	jobStore.EXPECT().
		UpdateJobRuntime(gomock.Any(), jobID, newJobRuntime).
		Return(nil)

	err := JobKill(context.Background(), jobEnt)
	assert.NoError(t, err)
}

func TestJobKillPartiallyCreatedJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	jobStore := storemocks.NewMockJobStore(ctrl)
	taskStore := storemocks.NewMockTaskStore(ctrl)
	jobGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	taskGoalStateEngine := goalstatemocks.NewMockEngine(ctrl)
	jobFactory := cachedmocks.NewMockJobFactory(ctrl)
	cachedJob := cachedmocks.NewMockJob(ctrl)

	goalStateDriver := &driver{
		jobEngine:  jobGoalStateEngine,
		taskEngine: taskGoalStateEngine,
		jobStore:   jobStore,
		taskStore:  taskStore,
		jobFactory: jobFactory,
		mtx:        NewMetrics(tally.NoopScope),
		cfg:        &Config{},
	}
	goalStateDriver.cfg.normalize()

	jobID := &peloton.JobID{Value: uuid.NewRandom().String()}

	jobEnt := &jobEntity{
		id:     jobID,
		driver: goalStateDriver,
	}

	instanceCount := uint32(4)
	jobConfig := pbjob.JobConfig{
		OwningTeam:    "team6",
		LdapGroups:    []string{"team1", "team2", "team3"},
		InstanceCount: instanceCount,
		Type:          pbjob.JobType_BATCH,
	}

	jobStore.EXPECT().
		GetJobConfig(gomock.Any(), jobID).
		Return(&jobConfig, nil)

	runtimes := make(map[uint32]*pbtask.RuntimeInfo)
	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_SUCCEEDED,
		GoalState: pbtask.TaskState_KILLED,
	}
	runtimes[3] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_KILLED,
		GoalState: pbtask.TaskState_KILLED,
	}
	jobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_INITIALIZED,
		GoalState: pbjob.JobState_KILLED,
	}
	newJobRuntime := &pbjob.RuntimeInfo{
		State:     pbjob.JobState_KILLED,
		GoalState: pbjob.JobState_KILLED,
	}

	taskStore.EXPECT().
		GetTaskRuntimesForJobByRange(gomock.Any(), jobID, gomock.Any()).
		Return(runtimes, nil)
	jobFactory.EXPECT().
		GetJob(jobID).
		Return(cachedJob)
	cachedJob.EXPECT().
		UpdateTasks(gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Return(nil)
	jobStore.EXPECT().
		GetJobRuntime(gomock.Any(), jobID).
		Return(jobRuntime, nil)
	cachedJob.EXPECT().
		IsPartiallyCreated().Return(true)
	jobStore.EXPECT().
		UpdateJobRuntime(gomock.Any(), jobID, newJobRuntime).
		Return(nil)

	err := JobKill(context.Background(), jobEnt)
	assert.NoError(t, err)

	runtimes[2] = &pbtask.RuntimeInfo{
		State:     pbtask.TaskState_RUNNING,
		GoalState: pbtask.TaskState_KILLED,
	}
	jobRuntime.State = pbjob.JobState_INITIALIZED
	newJobRuntime.State = pbjob.JobState_KILLING

	jobStore.EXPECT().
		GetJobConfig(gomock.Any(), jobID).
		Return(&jobConfig, nil)
	taskStore.EXPECT().
		GetTaskRuntimesForJobByRange(gomock.Any(), jobID, gomock.Any()).
		Return(runtimes, nil)
	jobFactory.EXPECT().
		GetJob(jobID).
		Return(cachedJob)
	cachedJob.EXPECT().
		UpdateTasks(gomock.Any(), gomock.Any(), cached.UpdateCacheAndDB).
		Return(nil)
	jobStore.EXPECT().
		GetJobRuntime(gomock.Any(), jobID).
		Return(jobRuntime, nil)
	cachedJob.EXPECT().
		IsPartiallyCreated().Return(true)
	jobStore.EXPECT().
		UpdateJobRuntime(gomock.Any(), jobID, newJobRuntime).
		Return(nil)

	err = JobKill(context.Background(), jobEnt)
	assert.NoError(t, err)
}