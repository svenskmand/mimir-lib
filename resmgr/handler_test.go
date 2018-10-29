package resmgr

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"code.uber.internal/infra/peloton/.gen/mesos/v1"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/peloton"
	pb_respool "code.uber.internal/infra/peloton/.gen/peloton/api/v0/respool"
	"code.uber.internal/infra/peloton/.gen/peloton/api/v0/task"
	pb_eventstream "code.uber.internal/infra/peloton/.gen/peloton/private/eventstream"
	host_mocks "code.uber.internal/infra/peloton/.gen/peloton/private/hostmgr/hostsvc/mocks"
	"code.uber.internal/infra/peloton/.gen/peloton/private/resmgr"
	"code.uber.internal/infra/peloton/.gen/peloton/private/resmgrsvc"

	"code.uber.internal/infra/peloton/common"
	"code.uber.internal/infra/peloton/common/eventstream"
	"code.uber.internal/infra/peloton/common/queue"
	"code.uber.internal/infra/peloton/common/statemachine"
	rc "code.uber.internal/infra/peloton/resmgr/common"
	"code.uber.internal/infra/peloton/resmgr/preemption/mocks"
	"code.uber.internal/infra/peloton/resmgr/respool"
	rm "code.uber.internal/infra/peloton/resmgr/respool/mocks"
	"code.uber.internal/infra/peloton/resmgr/scalar"
	rm_task "code.uber.internal/infra/peloton/resmgr/task"
	task_mocks "code.uber.internal/infra/peloton/resmgr/task/mocks"
	"code.uber.internal/infra/peloton/resmgr/tasktestutil"
	store_mocks "code.uber.internal/infra/peloton/storage/mocks"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/yarpc"
)

const (
	timeout = 1 * time.Second
)

type HandlerTestSuite struct {
	suite.Suite
	handler       *ServiceHandler
	context       context.Context
	resTree       respool.Tree
	taskScheduler rm_task.Scheduler
	ctrl          *gomock.Controller
	rmTaskTracker rm_task.Tracker
	cfg           rc.PreemptionConfig
	hostMgrClient *host_mocks.MockInternalHostServiceYARPCClient
}

func (s *HandlerTestSuite) SetupSuite() {
	s.ctrl = gomock.NewController(s.T())
	mockResPoolStore := store_mocks.NewMockResourcePoolStore(s.ctrl)
	mockResPoolStore.EXPECT().GetAllResourcePools(context.Background()).
		Return(s.getResPools(), nil).AnyTimes()
	mockJobStore := store_mocks.NewMockJobStore(s.ctrl)
	mockTaskStore := store_mocks.NewMockTaskStore(s.ctrl)

	s.cfg = rc.PreemptionConfig{
		Enabled: false,
	}

	respool.InitTree(tally.NoopScope, mockResPoolStore, mockJobStore,
		mockTaskStore, s.cfg)

	s.resTree = respool.GetTree()

	s.hostMgrClient = host_mocks.NewMockInternalHostServiceYARPCClient(s.ctrl)
	// Initializing the resmgr state machine
	rm_task.InitTaskTracker(
		tally.NoopScope,
		tasktestutil.CreateTaskConfig())

	s.rmTaskTracker = rm_task.GetTracker()
	rm_task.InitScheduler(tally.NoopScope, 1*time.Second, s.rmTaskTracker)
	s.taskScheduler = rm_task.GetScheduler()

	s.handler = &ServiceHandler{
		metrics:     NewMetrics(tally.NoopScope),
		resPoolTree: respool.GetTree(),
		placements: queue.NewQueue(
			"placement-queue",
			reflect.TypeOf(resmgr.Placement{}),
			maxPlacementQueueSize,
		),
		rmTracker: s.rmTaskTracker,
		config: Config{
			RmTaskConfig: tasktestutil.CreateTaskConfig(),
		},
	}
	s.handler.eventStreamHandler = eventstream.NewEventStreamHandler(
		1000,
		[]string{
			common.PelotonJobManager,
			common.PelotonResourceManager,
		},
		nil,
		tally.Scope(tally.NoopScope))
}

func (s *HandlerTestSuite) TearDownSuite() {
	s.ctrl.Finish()
	s.rmTaskTracker.Clear()
}

func (s *HandlerTestSuite) SetupTest() {
	s.context = context.Background()
	err := s.resTree.Start()
	s.NoError(err)

	err = s.taskScheduler.Start()
	s.NoError(err)

}

func (s *HandlerTestSuite) TearDownTest() {
	log.Info("tearing down")

	err := respool.GetTree().Stop()
	s.NoError(err)
	err = rm_task.GetScheduler().Stop()
	s.NoError(err)
}

func TestResManagerHandler(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) getResourceConfig() []*pb_respool.ResourceConfig {

	resConfigs := []*pb_respool.ResourceConfig{
		{
			Share:       1,
			Kind:        "cpu",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "memory",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "disk",
			Reservation: 100,
			Limit:       1000,
		},
		{
			Share:       1,
			Kind:        "gpu",
			Reservation: 2,
			Limit:       4,
		},
	}
	return resConfigs
}

func (s *HandlerTestSuite) getResPools() map[string]*pb_respool.ResourcePoolConfig {

	rootID := peloton.ResourcePoolID{Value: "root"}
	policy := pb_respool.SchedulingPolicy_PriorityFIFO

	return map[string]*pb_respool.ResourcePoolConfig{
		"root": {
			Name:      "root",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool1": {
			Name:      "respool1",
			Parent:    &rootID,
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool2": {
			Name:      "respool2",
			Parent:    &rootID,
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool3": {
			Name:      "respool3",
			Parent:    &rootID,
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool11": {
			Name:      "respool11",
			Parent:    &peloton.ResourcePoolID{Value: "respool1"},
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool12": {
			Name:      "respool12",
			Parent:    &peloton.ResourcePoolID{Value: "respool1"},
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool21": {
			Name:      "respool21",
			Parent:    &peloton.ResourcePoolID{Value: "respool2"},
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
		"respool22": {
			Name:      "respool22",
			Parent:    &peloton.ResourcePoolID{Value: "respool2"},
			Resources: s.getResourceConfig(),
			Policy:    policy,
		},
	}
}

func (s *HandlerTestSuite) pendingGang0() *resmgrsvc.Gang {
	var gang resmgrsvc.Gang
	uuidStr := "uuidstr-1"
	jobID := "job1"
	instance := 1
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	gang.Tasks = []*resmgr.Task{
		{
			Name:     "job1-1",
			Priority: 0,
			JobId:    &peloton.JobID{Value: "job1"},
			Id:       &peloton.TaskID{Value: fmt.Sprintf("%s-%d", jobID, instance)},
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			Preemptible:             true,
			PlacementTimeoutSeconds: 60,
			PlacementRetryCount:     1,
		},
	}
	return &gang
}

func (s *HandlerTestSuite) pendingGang1() *resmgrsvc.Gang {
	var gang resmgrsvc.Gang
	uuidStr := "uuidstr-1"
	jobID := "job1"
	instance := 2
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	gang.Tasks = []*resmgr.Task{
		{
			Name:     "job1-1",
			Priority: 1,
			JobId:    &peloton.JobID{Value: "job1"},
			Id:       &peloton.TaskID{Value: "job1-2"},
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			Preemptible: true,
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			PlacementTimeoutSeconds: 60,
			PlacementRetryCount:     1,
		},
	}
	return &gang
}

func (s *HandlerTestSuite) pendingGang2() *resmgrsvc.Gang {
	var gang resmgrsvc.Gang
	uuidStr := "uuidstr-1"
	jobID := "job1"
	instance := 2
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	gang.Tasks = []*resmgr.Task{
		{
			Name:         "job2-1",
			Priority:     2,
			MinInstances: 2,
			JobId:        &peloton.JobID{Value: "job2"},
			Id:           &peloton.TaskID{Value: "job2-1"},
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			Preemptible:             true,
			PlacementTimeoutSeconds: 60,
			PlacementRetryCount:     1,
		},
		{
			Name:         "job2-2",
			Priority:     2,
			MinInstances: 2,
			JobId:        &peloton.JobID{Value: "job2"},
			Id:           &peloton.TaskID{Value: "job2-2"},
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			Preemptible:             true,
			PlacementTimeoutSeconds: 60,
			PlacementRetryCount:     1,
		},
	}
	return &gang
}

func (s *HandlerTestSuite) pendingGangWithoutPlacement() *resmgrsvc.Gang {
	var gang resmgrsvc.Gang
	uuidStr := "uuidstr-1"
	jobID := "job9"
	instance := 1
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	gang.Tasks = []*resmgr.Task{
		{
			Name:     "job9-1",
			Priority: 0,
			JobId:    &peloton.JobID{Value: "job9"},
			Id:       &peloton.TaskID{Value: fmt.Sprintf("%s-%d", jobID, instance)},
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			Preemptible:             true,
			PlacementTimeoutSeconds: 60,
			PlacementRetryCount:     3,
		},
	}
	return &gang
}

func (s *HandlerTestSuite) pendingGangsWithoutPlacement() []*resmgrsvc.Gang {
	gangs := make([]*resmgrsvc.Gang, 1)
	gangs[0] = s.pendingGangWithoutPlacement()
	return gangs
}

func (s *HandlerTestSuite) pendingGangs() []*resmgrsvc.Gang {
	gangs := make([]*resmgrsvc.Gang, 3)
	gangs[0] = s.pendingGang0()
	gangs[1] = s.pendingGang1()
	gangs[2] = s.pendingGang2()
	return gangs
}

func (s *HandlerTestSuite) expectedGangs() []*resmgrsvc.Gang {
	gangs := make([]*resmgrsvc.Gang, 3)
	gangs[0] = s.pendingGang2()
	gangs[1] = s.pendingGang1()
	gangs[2] = s.pendingGang0()
	return gangs
}

func (s *HandlerTestSuite) TestNewServiceHandler() {
	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name:      common.PelotonResourceManager,
		Inbounds:  nil,
		Outbounds: nil,
		Metrics: yarpc.MetricsConfig{
			Tally: tally.NoopScope,
		},
	})

	tracker := task_mocks.NewMockTracker(s.ctrl)
	mockPreemptionQueue := mocks.NewMockQueue(s.ctrl)
	handler := NewServiceHandler(dispatcher, tally.NoopScope, tracker,
		mockPreemptionQueue, Config{})
	s.NotNil(handler)

	streamHandler := s.handler.GetStreamHandler()
	s.NotNil(streamHandler)
}

func (s *HandlerTestSuite) TestEnqueueDequeueGangsOneResPool() {
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   s.pendingGangs(),
	}
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	node.SetNonSlackEntitlement(s.getEntitlement())
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)

	s.NoError(err)
	s.Nil(enqResp.GetError())

	deqReq := &resmgrsvc.DequeueGangsRequest{
		Limit:   10,
		Timeout: 2 * 1000, // 2 sec
	}
	// There is a race condition in the test due to the Scheduler.scheduleTasks
	// method is run asynchronously.
	time.Sleep(2 * time.Second)

	deqResp, err := s.handler.DequeueGangs(s.context, deqReq)
	s.NoError(err)
	s.Nil(deqResp.GetError())
	s.Equal(s.expectedGangs(), deqResp.GetGangs())
}

func (s *HandlerTestSuite) TestReEnqueueGangNonExistingGangFails() {
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		Gangs: s.pendingGangs(),
	}
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.NotNil(enqResp.GetError())
	s.NotNil(enqResp.GetError().GetFailure().GetFailed())
}

func (s *HandlerTestSuite) TestReEnqueueGangThatFailedPlacement() {
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   s.pendingGangs(),
	}
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	node.SetNonSlackEntitlement(s.getEntitlement())
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())

	// There is a race condition in the test due to the Scheduler.scheduleTasks
	// method is run asynchronously.
	time.Sleep(2 * time.Second)

	// Re-enqueue the gangs without a resource pool
	enqReq.ResPool = nil
	enqResp, err = s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())

	// Make sure we dequeue the gangs again for the next test to work
	deqReq := &resmgrsvc.DequeueGangsRequest{
		Limit:   10,
		Timeout: 2 * 1000, // 2 sec
	}
	// Scheduler.scheduleTasks method is run asynchronously.
	// We need to wait here
	time.Sleep(timeout)
	// Checking whether we get the task from ready queue
	deqResp, err := s.handler.DequeueGangs(s.context, deqReq)
	s.NoError(err)
	s.Nil(deqResp.GetError())
}

func (s *HandlerTestSuite) TestReEnqueueGangThatFailedPlacementManyTimes() {
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   s.pendingGangsWithoutPlacement(),
	}
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	node.SetNonSlackEntitlement(s.getEntitlement())
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())

	// There is a race condition in the test due to the Scheduler.scheduleTasks
	// method is run asynchronously.
	time.Sleep(2 * time.Second)

	// Make sure we dequeue the gangs again for the next test to work
	deqReq := &resmgrsvc.DequeueGangsRequest{
		Limit:   10,
		Timeout: 2 * 1000, // 2 sec
	}
	// Scheduler.scheduleTasks method is run asynchronously.
	// We need to wait here
	time.Sleep(timeout)
	// Checking whether we get the task from ready queue
	deqResp, err := s.handler.DequeueGangs(s.context, deqReq)
	s.NoError(err)
	s.Nil(deqResp.GetError())

	// Re-enqueue the gangs without a resource pool
	enqReq.ResPool = nil
	enqResp, err = s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())

	rmTask := s.handler.rmTracker.GetTask(s.pendingGangsWithoutPlacement()[0].Tasks[0].Id)
	s.EqualValues(rmTask.GetCurrentState().String(), task.TaskState_PENDING.String())
}

// This tests the requeue of the same task with same mesos task id as well
// as the different mesos task id
func (s *HandlerTestSuite) TestRequeue() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	var gangs []*resmgrsvc.Gang
	gangs = append(gangs, s.pendingGang0())
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   gangs,
	}

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING})

	// Testing to see if we can send same task in the enqueue
	// request then it should error out
	node.SetNonSlackEntitlement(s.getEntitlement())
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.NotNil(enqResp.GetError())
	log.Error(err)
	log.Error(enqResp.GetError())
	s.EqualValues(enqResp.GetError().GetFailure().GetFailed()[0].Errorcode,
		resmgrsvc.EnqueueGangsFailure_ENQUEUE_GANGS_FAILURE_ERROR_CODE_ALREADY_EXIST)
	// Testing to see if we can send different Mesos taskID
	// in the enqueue request then it should move task to
	// ready state and ready queue
	uuidStr := "uuidstr-2"
	jobID := "job1"
	instance := 1
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	enqReq.Gangs[0].Tasks[0].TaskId = &mesos_v1.TaskID{
		Value: &mesosTaskID,
	}
	enqResp, err = s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())
	s.Nil(enqResp.GetError().GetFailure().GetFailed())

	rmtask = s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	s.EqualValues(rmtask.GetCurrentState(), task.TaskState_READY)

	deqReq := &resmgrsvc.DequeueGangsRequest{
		Limit:   10,
		Timeout: 2 * 1000, // 2 sec
	}
	// Checking whether we get the task from ready queue
	deqResp, err := s.handler.DequeueGangs(s.context, deqReq)
	s.NoError(err)
	s.Nil(deqResp.GetError())
	log.Info(*deqResp.GetGangs()[0].Tasks[0].TaskId.Value)
	s.Equal(mesosTaskID, *deqResp.GetGangs()[0].Tasks[0].TaskId.Value)
}

// TestRequeueTaskNotPresent tests the requeue but if the task is been
// removed from the tracker then result should be failed
func (s *HandlerTestSuite) TestRequeueTaskNotPresent() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING})
	s.rmTaskTracker.DeleteTask(s.pendingGang0().Tasks[0].Id)
	failed, err := s.handler.requeueTask(s.pendingGang0().Tasks[0])
	s.Error(err)
	s.NotNil(failed)
	s.EqualValues(failed.Errorcode, resmgrsvc.EnqueueGangsFailure_ENQUEUE_GANGS_FAILURE_ERROR_CODE_INTERNAL)
}

func (s *HandlerTestSuite) TestRequeueFailures() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   []*resmgrsvc.Gang{s.pendingGang0()},
	}

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING})
	// Testing to see if we can send same task in the enqueue
	// request then it should error out
	node.SetNonSlackEntitlement(s.getEntitlement())
	// Testing to see if we can send different Mesos taskID
	// in the enqueue request then it should move task to
	// ready state and ready queue
	uuidStr := "uuidstr-2"
	jobID := "job1"
	instance := 1
	mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, instance, uuidStr)
	enqReq.Gangs[0].Tasks[0].TaskId = &mesos_v1.TaskID{
		Value: &mesosTaskID,
	}
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_RUNNING,
		task.TaskState_SUCCEEDED})
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.NotNil(enqResp.GetError())
	s.EqualValues(enqResp.GetError().GetFailure().GetFailed()[0].Errorcode,
		resmgrsvc.EnqueueGangsFailure_ENQUEUE_GANGS_FAILURE_ERROR_CODE_INTERNAL)
}

func (s *HandlerTestSuite) TestAddingToPendingQueue() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED})
	err = s.handler.addingGangToPendingQueue(s.pendingGang0(), node)
	s.Error(err)
	s.EqualValues(err.Error(), errGangNotEnqueued.Error())
}

func (s *HandlerTestSuite) TestAddingToPendingQueueFailure() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED})
	err = s.handler.addingGangToPendingQueue(&resmgrsvc.Gang{}, node)
	s.Error(err)
	s.EqualValues(err.Error(), errGangNotEnqueued.Error())
	s.rmTaskTracker.Clear()
}

func (s *HandlerTestSuite) TestRequeuePlacementFailure() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED})
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: nil,
		Gangs:   []*resmgrsvc.Gang{s.pendingGang0()},
	}

	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.NotNil(enqResp.GetError())
}

func (s *HandlerTestSuite) TestEnqueueGangsResPoolNotFound() {
	respool.InitTree(tally.NoopScope, nil, nil, nil, s.cfg)

	respoolID := &peloton.ResourcePoolID{Value: "respool10"}
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: respoolID,
		Gangs:   s.pendingGangs(),
	}
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	log.Infof("%v", enqResp)
	notFound := &resmgrsvc.ResourcePoolNotFound{
		Id:      respoolID,
		Message: "resource pool (respool10) not found",
	}
	s.Equal(notFound, enqResp.GetError().GetNotFound())
}

func (s *HandlerTestSuite) TestEnqueueGangsFailure() {
	// TODO: Mock ResPool.Enqueue task to simulate task enqueue failures
	s.True(true)
}

func (s *HandlerTestSuite) getPlacements() []*resmgr.Placement {
	var placements []*resmgr.Placement
	resp, err := respool.NewRespool(
		tally.NoopScope,
		"respool-1",
		nil,
		&pb_respool.ResourcePoolConfig{
			Policy: pb_respool.SchedulingPolicy_PriorityFIFO,
		},
		s.cfg,
	)
	s.NoError(err, "create resource pool shouldn't fail")

	for i := 0; i < 10; i++ {
		var tasks []*peloton.TaskID
		for j := 0; j < 5; j++ {
			task := &peloton.TaskID{
				Value: fmt.Sprintf("task-%d-%d", i, j),
			}
			tasks = append(tasks, task)
			s.rmTaskTracker.AddTask(&resmgr.Task{
				Id: task,
			}, nil, resp, tasktestutil.CreateTaskConfig())
		}

		placement := &resmgr.Placement{
			Tasks:    tasks,
			Hostname: fmt.Sprintf("host-%d", i),
		}
		placements = append(placements, placement)
	}
	return placements
}

func (s *HandlerTestSuite) TestSetAndGetPlacementsSuccess() {
	handler := &ServiceHandler{
		metrics:     NewMetrics(tally.NoopScope),
		resPoolTree: nil,
		placements: queue.NewQueue(
			"placement-queue",
			reflect.TypeOf(resmgr.Placement{}),
			maxPlacementQueueSize,
		),
		rmTracker: s.rmTaskTracker,
	}
	handler.eventStreamHandler = s.handler.eventStreamHandler

	setReq := &resmgrsvc.SetPlacementsRequest{
		Placements: s.getPlacements(),
	}
	for _, placement := range setReq.Placements {
		for _, taskID := range placement.Tasks {
			rmTask := handler.rmTracker.GetTask(taskID)
			tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
				task.TaskState_PENDING,
				task.TaskState_READY,
				task.TaskState_PLACING})
		}
	}
	setResp, err := handler.SetPlacements(s.context, setReq)
	s.NoError(err)
	s.Nil(setResp.GetError())

	getReq := &resmgrsvc.GetPlacementsRequest{
		Limit:   10,
		Timeout: 1 * 1000, // 1 sec
	}
	getResp, err := handler.GetPlacements(s.context, getReq)
	s.NoError(err)
	s.Nil(getResp.GetError())
	s.Equal(s.getPlacements(), getResp.GetPlacements())
}

func (s *HandlerTestSuite) TestTransitTasksInPlacement() {

	tracker := task_mocks.NewMockTracker(s.ctrl)
	s.handler.rmTracker = tracker

	placements := s.getPlacements()

	tracker.EXPECT().GetTask(gomock.Any()).Return(nil).Times(5)

	p := s.handler.transitTasksInPlacement(placements[0],
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING,
		"placement dequeued, waiting for launch")

	s.EqualValues(len(p.Tasks), 0)

	resp, err := respool.NewRespool(tally.NoopScope, "respool-1", nil, &pb_respool.ResourcePoolConfig{
		Name:      "respool-1",
		Parent:    nil,
		Resources: s.getResourceConfig(),
		Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
	},
		s.cfg)
	uuidStr := uuid.NewUUID().String()
	t := &resmgr.Task{
		Id: &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", uuidStr, 0),
		},
	}
	rmTask, err := rm_task.CreateRMTask(t, nil,
		resp,
		rm_task.DefaultTransitionObserver(
			tally.NoopScope,
			t,
			resp,
		),
		&rm_task.Config{
			LaunchingTimeout: 1 * time.Minute,
			PlacingTimeout:   1 * time.Minute,
			PolicyName:       rm_task.ExponentialBackOffPolicy,
		},
	)
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
		task.TaskState_PENDING,
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
	})

	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask).Times(5)
	placements = s.getPlacements()
	p = s.handler.transitTasksInPlacement(placements[0],
		task.TaskState_RUNNING,
		task.TaskState_LAUNCHING,
		"placement dequeued, waiting for launch")
	s.EqualValues(len(p.Tasks), 0)

	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask).Times(5)
	placements = s.getPlacements()
	p = s.handler.transitTasksInPlacement(placements[0],
		task.TaskState_PLACED,
		task.TaskState_RUNNING,
		"placement dequeued, waiting for launch")
	s.EqualValues(len(p.Tasks), 0)

	s.handler.rmTracker = rm_task.GetTracker()
	s.rmTaskTracker.Clear()
}

func (s *HandlerTestSuite) TestGetTasksByHosts() {
	setReq := &resmgrsvc.SetPlacementsRequest{
		Placements: s.getPlacements(),
	}
	hostnames := make([]string, 0, len(setReq.Placements))
	for _, placement := range setReq.Placements {
		hostnames = append(hostnames, placement.Hostname)
		for _, taskID := range placement.Tasks {
			rmTask := s.handler.rmTracker.GetTask(taskID)
			tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
				task.TaskState_PENDING,
				task.TaskState_READY,
				task.TaskState_PLACING})
		}
	}
	setResp, err := s.handler.SetPlacements(s.context, setReq)
	s.NoError(err)
	s.Nil(setResp.GetError())

	req := &resmgrsvc.GetTasksByHostsRequest{
		Hostnames: hostnames,
	}
	res, err := s.handler.GetTasksByHosts(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	s.Equal(len(hostnames), len(res.HostTasksMap))
	for _, hostname := range hostnames {
		_, exists := res.HostTasksMap[hostname]
		s.True(exists)
	}
	for _, placement := range setReq.Placements {
		s.Equal(len(placement.Tasks), len(res.HostTasksMap[placement.Hostname].Tasks))
	}
}

func (s *HandlerTestSuite) TestRemoveTasksFromPlacement() {
	_, tasks := s.createRMTasks()
	placement := &resmgr.Placement{
		Tasks:    tasks,
		Hostname: fmt.Sprintf("host-%d", 1),
	}
	s.Equal(len(placement.Tasks), 5)
	taskstoremove := make(map[string]*peloton.TaskID)
	for j := 0; j < 2; j++ {
		taskID := &peloton.TaskID{
			Value: fmt.Sprintf("task-1-%d", j),
		}
		taskstoremove[taskID.Value] = taskID
	}
	newPlacement := s.handler.removeTasksFromPlacements(placement, taskstoremove)
	s.NotNil(newPlacement)
	s.Equal(len(newPlacement.Tasks), 3)
}

func (s *HandlerTestSuite) TestRemoveTasksFromGang() {
	rmtasks, _ := s.createRMTasks()
	gang := &resmgrsvc.Gang{
		Tasks: rmtasks,
	}
	s.Equal(len(gang.Tasks), 5)
	tasksToRemove := make(map[string]*resmgr.Task)
	tasksToRemove[rmtasks[0].Id.Value] = rmtasks[0]
	tasksToRemove[rmtasks[1].Id.Value] = rmtasks[1]
	newGang := s.handler.removeFromGang(gang, tasksToRemove)
	s.NotNil(newGang)
	s.Equal(len(newGang.Tasks), 3)
}

func (s *HandlerTestSuite) createRMTasks() ([]*resmgr.Task, []*peloton.TaskID) {
	var tasks []*peloton.TaskID
	var rmTasks []*resmgr.Task
	resp, _ := respool.NewRespool(tally.NoopScope, "respool-1", nil,
		&pb_respool.ResourcePoolConfig{
			Name:      "respool1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)
	for j := 0; j < 5; j++ {
		mesosTaskID := "mesosID"
		taskID := &peloton.TaskID{
			Value: fmt.Sprintf("task-1-%d", j),
		}
		tasks = append(tasks, taskID)
		rmTask := &resmgr.Task{
			Id: taskID,
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
		}
		s.rmTaskTracker.AddTask(rmTask, nil, resp,
			tasktestutil.CreateTaskConfig())
		rmTasks = append(rmTasks, rmTask)
	}
	return rmTasks, tasks
}

func (s *HandlerTestSuite) TestKillTasks() {
	s.rmTaskTracker.Clear()
	_, tasks := s.createRMTasks()

	var killedtasks []*peloton.TaskID
	killedtasks = append(killedtasks, tasks[0])
	killedtasks = append(killedtasks, tasks[1])

	killReq := &resmgrsvc.KillTasksRequest{
		Tasks: killedtasks,
	}
	// This is a valid list tasks should be deleted
	// Result is no error and tracker should have remaining 3 tasks
	res, err := s.handler.KillTasks(s.context, killReq)
	s.NoError(err)
	s.Nil(res.Error)
	s.Equal(s.rmTaskTracker.GetSize(), int64(3))
	var notValidkilledtasks []*peloton.TaskID
	killReq = &resmgrsvc.KillTasksRequest{
		Tasks: notValidkilledtasks,
	}
	// This list does not have any tasks in the list
	// this should return error.
	res, err = s.handler.KillTasks(s.context, killReq)
	s.NotNil(res.Error)
	notValidkilledtasks = append(notValidkilledtasks, tasks[0])
	killReq = &resmgrsvc.KillTasksRequest{
		Tasks: notValidkilledtasks,
	}
	// This list have invalid task in the list which should be not
	// present in the tracker and should return error
	res, err = s.handler.KillTasks(s.context, killReq)
	s.NotNil(res.Error)
	s.NotNil(res.Error[0].NotFound)
	s.Nil(res.Error[0].KillError)
	s.Equal(res.Error[0].NotFound.Task.Value, tasks[0].Value)
}

// TestUpdateTasksState tests the update tasks by state
func (s *HandlerTestSuite) TestUpdateTasksState() {
	s.rmTaskTracker.Clear()
	// Creating the New set of Tasks and add them to the tracker
	rmTasks, _ := s.createRMTasks()
	invalidMesosID := "invalid_mesos_id"

	testTable := []struct {
		updateStateRequestTask *resmgr.Task
		updatedTaskState       task.TaskState
		mesosTaskID            *mesos_v1.TaskID
		expectedState          task.TaskState
		expectedTask           string
		msg                    string
		deletefromTracker      bool
	}{
		{
			msg: "Testing valid tasks, test if the state reached to LAUNCHED",
			updateStateRequestTask: rmTasks[0],
			updatedTaskState:       task.TaskState_UNKNOWN,
			mesosTaskID:            nil,
			expectedState:          task.TaskState_LAUNCHED,
			expectedTask:           "",
		},
		{
			msg: "Testing update the same task, It should not update the Launched state",
			updateStateRequestTask: rmTasks[0],
			updatedTaskState:       task.TaskState_UNKNOWN,
			mesosTaskID:            nil,
			expectedState:          task.TaskState_LAUNCHED,
			expectedTask:           "",
		},
		{
			msg: "Testing if we pass the nil Resource Manager Task",
			updateStateRequestTask: nil,
			updatedTaskState:       task.TaskState_UNKNOWN,
			mesosTaskID:            nil,
			expectedState:          task.TaskState_UNKNOWN,
			expectedTask:           "",
		},
		{
			msg: "Testing RMtask with invalid mesos task id with Terminal state",
			updateStateRequestTask: rmTasks[0],
			updatedTaskState:       task.TaskState_KILLED,
			mesosTaskID: &mesos_v1.TaskID{
				Value: &invalidMesosID,
			},
			expectedState: task.TaskState_UNKNOWN,
			expectedTask:  "",
		},
		{
			msg: "Testing RMtask with invalid mesos task id.",
			updateStateRequestTask: rmTasks[0],
			updatedTaskState:       task.TaskState_UNKNOWN,
			mesosTaskID: &mesos_v1.TaskID{
				Value: &invalidMesosID,
			},
			expectedState: task.TaskState_UNKNOWN,
			expectedTask:  "not_nil",
		},
		{
			msg: "Testing RMtask with Terminal State",
			updateStateRequestTask: rmTasks[0],
			updatedTaskState:       task.TaskState_KILLED,
			mesosTaskID:            nil,
			expectedState:          task.TaskState_UNKNOWN,
			expectedTask:           "nil",
		},
		{
			msg: "Testing RMtask with Task deleted from tracker",
			updateStateRequestTask: rmTasks[1],
			updatedTaskState:       task.TaskState_UNKNOWN,
			mesosTaskID:            nil,
			expectedState:          task.TaskState_UNKNOWN,
			expectedTask:           "nil",
			deletefromTracker:      true,
		},
	}

	for _, tt := range testTable {
		if tt.deletefromTracker {
			s.rmTaskTracker.DeleteTask(tt.updateStateRequestTask.Id)
		}
		req := s.createUpdateTasksStateRequest(tt.updateStateRequestTask)
		if tt.mesosTaskID != nil {
			req.GetTaskStates()[0].MesosTaskId = tt.mesosTaskID
		}
		if tt.updatedTaskState != task.TaskState_UNKNOWN {
			req.GetTaskStates()[0].State = tt.updatedTaskState
		}
		_, err := s.handler.UpdateTasksState(s.context, req)
		s.NoError(err)
		if tt.expectedState != task.TaskState_UNKNOWN {
			s.Equal(tt.expectedState, s.rmTaskTracker.GetTask(tt.updateStateRequestTask.Id).GetCurrentState())
		}
		switch tt.expectedTask {
		case "nil":
			s.Nil(s.rmTaskTracker.GetTask(tt.updateStateRequestTask.Id))
		case "not_nil":
			s.NotNil(s.rmTaskTracker.GetTask(tt.updateStateRequestTask.Id))
		default:
			break
		}
	}
}

// createUpdateTasksStateRequest creates UpdateTaskstate Request
// based on the resource manager task specified
func (s *HandlerTestSuite) createUpdateTasksStateRequest(
	rmTask *resmgr.Task,
) *resmgrsvc.UpdateTasksStateRequest {
	taskList := make([]*resmgrsvc.UpdateTasksStateRequest_UpdateTaskStateEntry, 0, 1)
	if rmTask != nil {
		taskList = append(taskList, &resmgrsvc.UpdateTasksStateRequest_UpdateTaskStateEntry{
			State:       task.TaskState_LAUNCHED,
			MesosTaskId: rmTask.GetTaskId(),
			Task:        rmTask.GetId(),
		})
	}
	return &resmgrsvc.UpdateTasksStateRequest{
		TaskStates: taskList,
	}
}

func (s *HandlerTestSuite) TestNotifyTaskStatusUpdate() {
	var c uint64
	rm_task.InitTaskTracker(
		tally.NoopScope,
		tasktestutil.CreateTaskConfig())
	handler := &ServiceHandler{
		metrics:   NewMetrics(tally.NoopScope),
		maxOffset: &c,
		rmTracker: rm_task.GetTracker(),
	}
	jobID := "test"
	uuidStr := uuid.NewUUID().String()
	var events []*pb_eventstream.Event
	resp, _ := respool.NewRespool(tally.NoopScope, "respool-1", nil,
		&pb_respool.ResourcePoolConfig{
			Name:      "respool1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)
	for i := 0; i < 100; i++ {
		mesosTaskID := fmt.Sprintf("%s-%d-%s", jobID, i, uuidStr)
		state := mesos_v1.TaskState_TASK_FINISHED
		status := &mesos_v1.TaskStatus{
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			State: &state,
		}
		event := pb_eventstream.Event{
			Offset:          uint64(1000 + i),
			MesosTaskStatus: status,
		}
		events = append(events, &event)
		ptask := &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", jobID, i),
		}

		handler.rmTracker.AddTask(&resmgr.Task{
			Id: ptask,
			Resource: &task.ResourceConfig{
				CpuLimit:    1,
				DiskLimitMb: 10,
				GpuLimit:    0,
				MemLimitMb:  100,
			},
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
		}, nil, resp, tasktestutil.CreateTaskConfig())
	}
	req := &resmgrsvc.NotifyTaskUpdatesRequest{
		Events: events,
	}
	response, _ := handler.NotifyTaskUpdates(context.Background(), req)
	assert.Equal(s.T(), uint64(1099), response.PurgeOffset)
	assert.Nil(s.T(), response.Error)
}

func (s *HandlerTestSuite) TestHandleEventError() {
	tracker := task_mocks.NewMockTracker(s.ctrl)
	s.handler.rmTracker = tracker

	var c uint64
	s.handler.maxOffset = &c

	uuidStr := uuid.NewUUID().String()
	var events []*pb_eventstream.Event

	for i := 0; i < 1; i++ {
		mesosTaskID := fmt.Sprintf("%s-%d-%s", uuidStr, i, uuidStr)
		state := mesos_v1.TaskState_TASK_FINISHED
		status := &mesos_v1.TaskStatus{
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			State: &state,
		}
		event := pb_eventstream.Event{
			Offset:          uint64(1000 + i),
			MesosTaskStatus: status,
		}
		events = append(events, &event)
	}

	req := &resmgrsvc.NotifyTaskUpdatesRequest{}
	// testing empty events
	response, err := s.handler.NotifyTaskUpdates(context.Background(), req)
	s.NoError(err)

	req = &resmgrsvc.NotifyTaskUpdatesRequest{
		Events: events,
	}

	tracker.EXPECT().GetTask(gomock.Any()).Return(nil)
	response, _ = s.handler.NotifyTaskUpdates(context.Background(), req)

	s.EqualValues(uint64(1000), response.PurgeOffset)
	s.Nil(response.Error)

	resp, _ := respool.NewRespool(
		tally.NoopScope, "respool-1", nil, &pb_respool.ResourcePoolConfig{
			Name:      "respool-1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)

	mesosTaskID := fmt.Sprintf("%s-%d-%s", uuidStr, 0, uuidStr)
	t := &resmgr.Task{
		Id: &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", uuidStr, 0),
		},
		TaskId: &mesos_v1.TaskID{
			Value: &mesosTaskID,
		},
	}
	rmTask, err := rm_task.CreateRMTask(t, nil,
		resp,
		rm_task.DefaultTransitionObserver(
			tally.NoopScope,
			t,
			resp,
		),
		&rm_task.Config{
			LaunchingTimeout: 1 * time.Minute,
			PlacingTimeout:   1 * time.Minute,
			PolicyName:       rm_task.ExponentialBackOffPolicy,
		},
	)
	s.NoError(err)

	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask)
	tracker.EXPECT().MarkItDone(gomock.Any(), gomock.Any()).Return(nil)
	response, _ = s.handler.NotifyTaskUpdates(context.Background(), req)
	s.EqualValues(uint64(1000), response.PurgeOffset)
	s.Nil(response.Error)

	// Testing wrong mesos task id
	wmesosTaskID := fmt.Sprintf("%s-%d-%s", "job1", 0, uuidStr)
	t = &resmgr.Task{
		Id: &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", uuidStr, 0),
		},
		TaskId: &mesos_v1.TaskID{
			Value: &wmesosTaskID,
		},
	}
	wrmTask, err := rm_task.CreateRMTask(
		t,
		nil,
		resp,
		rm_task.DefaultTransitionObserver(
			tally.NoopScope,
			t,
			resp,
		),
		&rm_task.Config{
			LaunchingTimeout: 1 * time.Minute,
			PlacingTimeout:   1 * time.Minute,
			PolicyName:       rm_task.ExponentialBackOffPolicy,
		},
	)
	s.NoError(err)

	tracker.EXPECT().GetTask(gomock.Any()).Return(wrmTask)
	response, _ = s.handler.NotifyTaskUpdates(context.Background(), req)
	s.EqualValues(uint64(1000), response.PurgeOffset)
	s.Nil(response.Error)

	// Testing with markitdone error
	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask)
	tracker.EXPECT().MarkItDone(gomock.Any(), gomock.Any()).Return(errors.New("error"))
	response, _ = s.handler.NotifyTaskUpdates(context.Background(), req)
	s.EqualValues(uint64(1000), response.PurgeOffset)
	s.Nil(response.Error)

	s.handler.rmTracker = rm_task.GetTracker()
}

func (s *HandlerTestSuite) TestHandleRunningEventError() {
	tracker := task_mocks.NewMockTracker(s.ctrl)
	s.handler.rmTracker = tracker

	var c uint64
	s.handler.maxOffset = &c

	uuidStr := uuid.NewUUID().String()
	var events []*pb_eventstream.Event

	for i := 0; i < 1; i++ {
		mesosTaskID := fmt.Sprintf("%s-%d-%s", uuidStr, i, uuidStr)
		state := mesos_v1.TaskState_TASK_RUNNING
		status := &mesos_v1.TaskStatus{
			TaskId: &mesos_v1.TaskID{
				Value: &mesosTaskID,
			},
			State: &state,
		}
		event := pb_eventstream.Event{
			Offset:          uint64(1000 + i),
			MesosTaskStatus: status,
		}
		events = append(events, &event)
	}

	req := &resmgrsvc.NotifyTaskUpdatesRequest{}

	req = &resmgrsvc.NotifyTaskUpdatesRequest{
		Events: events,
	}

	// Testing with task state running
	resp, _ := respool.NewRespool(
		tally.NoopScope, "respool-1", nil, &pb_respool.ResourcePoolConfig{
			Name:      "respool-1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)

	mesosTaskID := fmt.Sprintf("%s-%d-%s", uuidStr, 0, uuidStr)

	t := &resmgr.Task{
		Id: &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", uuidStr, 0),
		},
		TaskId: &mesos_v1.TaskID{
			Value: &mesosTaskID,
		},
	}
	rmTask, err := rm_task.CreateRMTask(
		t,
		nil,
		resp,
		rm_task.DefaultTransitionObserver(
			tally.NoopScope,
			t,
			resp,
		),
		&rm_task.Config{
			LaunchingTimeout: 1 * time.Minute,
			PlacingTimeout:   1 * time.Minute,
			PolicyName:       rm_task.ExponentialBackOffPolicy,
		},
	)
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
		task.TaskState_PENDING,
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING,
		task.TaskState_RUNNING,
	})

	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask)
	response, _ := s.handler.NotifyTaskUpdates(context.Background(), req)

	s.EqualValues(uint64(1000), response.PurgeOffset)
	s.Nil(response.Error)

	s.handler.rmTracker = rm_task.GetTracker()

}

func (s *HandlerTestSuite) getEntitlement() *scalar.Resources {
	return &scalar.Resources{
		CPU:    100,
		MEMORY: 1000,
		DISK:   100,
		GPU:    2,
	}
}

func (s *HandlerTestSuite) TestGetActiveTasks() {
	setReq := &resmgrsvc.SetPlacementsRequest{
		Placements: s.getPlacements(),
	}
	for _, placement := range setReq.Placements {
		for _, taskID := range placement.Tasks {
			rmTask := s.handler.rmTracker.GetTask(taskID)
			tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
				task.TaskState_PENDING,
				task.TaskState_READY,
				task.TaskState_PLACING})
		}
	}
	setResp, err := s.handler.SetPlacements(s.context, setReq)
	s.NoError(err)
	s.Nil(setResp.GetError())

	req := &resmgrsvc.GetActiveTasksRequest{}
	res, err := s.handler.GetActiveTasks(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	totalTasks := 0
	for _, tasks := range res.GetTasksByState() {
		totalTasks += len(tasks.GetTaskEntry())
	}
	s.Equal(54, totalTasks)
}

func (s *HandlerTestSuite) TestGetPreemptibleTasks() {
	defer s.handler.rmTracker.Clear()

	mockPreemptionQueue := mocks.NewMockQueue(s.ctrl)
	s.handler.preemptionQueue = mockPreemptionQueue

	// Mock tasks in RUNNING state
	resp, err := respool.NewRespool(
		tally.NoopScope,
		"respool-1",
		nil,
		&pb_respool.ResourcePoolConfig{
			Policy: pb_respool.SchedulingPolicy_PriorityFIFO,
		},
		s.cfg,
	)
	s.NoError(err, "create resource pool should not fail")

	var expectedTasks []*resmgr.Task
	for j := 1; j <= 5; j++ {
		taskID := &peloton.TaskID{
			Value: fmt.Sprintf("task-test-dequque-preempt-%d-%d", j, j),
		}
		expectedTasks = append(expectedTasks, &resmgr.Task{
			Id: taskID,
		})
		s.rmTaskTracker.AddTask(&resmgr.Task{
			Id: taskID,
		}, nil, resp,
			tasktestutil.CreateTaskConfig())
		rmTask := s.handler.rmTracker.GetTask(taskID)
		tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
			task.TaskState_PENDING,
			task.TaskState_READY,
			task.TaskState_PLACING,
			task.TaskState_PLACED,
			task.TaskState_LAUNCHING,
			task.TaskState_RUNNING,
		})
	}

	var calls []*gomock.Call
	for _, et := range expectedTasks {
		calls = append(calls, mockPreemptionQueue.
			EXPECT().
			DequeueTask(gomock.Any()).
			Return(&resmgr.PreemptionCandidate{
				Id:     et.Id,
				Reason: resmgr.PreemptionReason_PREEMPTION_REASON_REVOKE_RESOURCES,
			}, nil))
	}
	gomock.InOrder(calls...)

	// Make RPC request
	req := &resmgrsvc.GetPreemptibleTasksRequest{
		Timeout: 100,
		Limit:   5,
	}
	res, err := s.handler.GetPreemptibleTasks(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	s.Equal(5, len(res.PreemptionCandidates))
}

func (s *HandlerTestSuite) TestGetPreemptibleTasksError() {
	tracker := task_mocks.NewMockTracker(s.ctrl)
	mockPreemptionQueue := mocks.NewMockQueue(s.ctrl)
	s.handler.preemptionQueue = mockPreemptionQueue
	s.handler.rmTracker = tracker

	// Mock tasks in RUNNING state
	resp, _ := respool.NewRespool(
		tally.NoopScope, "respool-1", nil, &pb_respool.ResourcePoolConfig{
			Name:      "respool-1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)
	t := &resmgr.Task{
		Id: &peloton.TaskID{
			Value: fmt.Sprintf("%s-%d", "job1-", 0),
		},
	}
	rmTask, err := rm_task.CreateRMTask(
		t,
		nil,
		resp,
		rm_task.DefaultTransitionObserver(
			tally.NoopScope,
			t,
			resp,
		),
		&rm_task.Config{
			LaunchingTimeout: 1 * time.Minute,
			PlacingTimeout:   1 * time.Minute,
			PolicyName:       rm_task.ExponentialBackOffPolicy,
		},
	)

	s.NoError(err)
	tracker.EXPECT().AddTask(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any()).
		Return(nil)

	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask)

	var expectedTasks []*resmgr.Task
	for j := 1; j <= 1; j++ {
		taskID := &peloton.TaskID{
			Value: fmt.Sprintf("task-test-dequque-preempt-%d-%d", j, j),
		}
		expectedTasks = append(expectedTasks, &resmgr.Task{
			Id: taskID,
		})
		tracker.AddTask(&resmgr.Task{
			Id: taskID,
		}, nil, resp,
			tasktestutil.CreateTaskConfig())
		rmTask := tracker.GetTask(taskID)
		tasktestutil.ValidateStateTransitions(rmTask, []task.TaskState{
			task.TaskState_PENDING,
			task.TaskState_READY,
			task.TaskState_PLACING,
			task.TaskState_PLACED,
			task.TaskState_LAUNCHING,
		})
	}

	mockPreemptionQueue.EXPECT().DequeueTask(gomock.Any()).Return(nil, errors.New("error"))

	// Make RPC request
	req := &resmgrsvc.GetPreemptibleTasksRequest{
		Timeout: 100,
		Limit:   1,
	}
	res, err := s.handler.GetPreemptibleTasks(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	s.Equal(0, len(res.PreemptionCandidates))

	mockPreemptionQueue.EXPECT().DequeueTask(gomock.Any()).Return(&resmgr.PreemptionCandidate{
		Id:     expectedTasks[0].Id,
		Reason: resmgr.PreemptionReason_PREEMPTION_REASON_REVOKE_RESOURCES,
	}, nil)
	tracker.EXPECT().GetTask(gomock.Any()).Return(rmTask)
	res, err = s.handler.GetPreemptibleTasks(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	s.Equal(0, len(res.PreemptionCandidates))

	mockPreemptionQueue.EXPECT().DequeueTask(gomock.Any()).Return(&resmgr.PreemptionCandidate{
		Id:     expectedTasks[0].Id,
		Reason: resmgr.PreemptionReason_PREEMPTION_REASON_REVOKE_RESOURCES,
	}, nil)
	tracker.EXPECT().GetTask(gomock.Any()).Return(nil)
	res, err = s.handler.GetPreemptibleTasks(context.Background(), req)
	s.NoError(err)
	s.NotNil(res)
	s.Equal(0, len(res.PreemptionCandidates))
	s.handler.rmTracker = rm_task.GetTracker()
}

func (s *HandlerTestSuite) TestAddTaskError() {
	tracker := task_mocks.NewMockTracker(s.ctrl)
	s.handler.rmTracker = tracker

	tracker.EXPECT().AddTask(
		gomock.Any(),
		gomock.Any(),
		gomock.Any(),
		gomock.Any()).
		Return(errors.New("error"))
	resp, _ := respool.NewRespool(
		tally.NoopScope, "respool-1", nil, &pb_respool.ResourcePoolConfig{
			Name:      "respool-1",
			Parent:    nil,
			Resources: s.getResourceConfig(),
			Policy:    pb_respool.SchedulingPolicy_PriorityFIFO,
		}, s.cfg)

	response, err := s.handler.addTask(&resmgr.Task{}, resp)
	s.Error(err)
	s.Equal(response.Message, "error")
	s.handler.rmTracker = rm_task.GetTracker()
}

func (s *HandlerTestSuite) TestRequeueInvalidatedTasks() {
	node, err := s.resTree.Get(&peloton.ResourcePoolID{Value: "respool3"})
	s.NoError(err)
	enqReq := &resmgrsvc.EnqueueGangsRequest{
		ResPool: &peloton.ResourcePoolID{Value: "respool3"},
		Gangs:   []*resmgrsvc.Gang{s.pendingGang0()},
	}

	s.rmTaskTracker.AddTask(
		s.pendingGang0().Tasks[0],
		nil,
		node,
		tasktestutil.CreateTaskConfig())
	rmtask := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	err = rmtask.TransitTo(task.TaskState_PENDING.String(), statemachine.WithInfo(mesosTaskID,
		*s.pendingGang0().Tasks[0].TaskId.Value))
	s.NoError(err)
	tasktestutil.ValidateStateTransitions(rmtask, []task.TaskState{
		task.TaskState_READY,
		task.TaskState_PLACING,
		task.TaskState_PLACED,
		task.TaskState_LAUNCHING,
	})

	// Marking this task to Invalidate
	// It will not invalidate as its in Lunching state
	s.rmTaskTracker.MarkItInvalid(s.pendingGang0().Tasks[0].Id, *s.pendingGang0().Tasks[0].TaskId.Value)

	// Tasks should be removed from Tracker
	taskget := s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	s.Nil(taskget)

	// Testing to see if we can send same task in the enqueue
	// after invalidating the task
	node.SetNonSlackEntitlement(s.getEntitlement())
	enqResp, err := s.handler.EnqueueGangs(s.context, enqReq)
	s.NoError(err)
	s.Nil(enqResp.GetError())
	// Waiting for scheduler to kick in, As the task was not
	// in initialized and pending state it will not be invalidate
	// and should be able to requeue and get to READY state
	time.Sleep(timeout)
	rmtask = s.rmTaskTracker.GetTask(s.pendingGang0().Tasks[0].Id)
	s.EqualValues(rmtask.GetCurrentState(), task.TaskState_READY)
}

func (s *HandlerTestSuite) TestGetPendingTasks() {
	respoolID := &peloton.ResourcePoolID{Value: "respool3"}
	limit := uint32(1)

	mr := rm.NewMockResPool(s.ctrl)
	mr.EXPECT().IsLeaf().Return(true)
	mr.EXPECT().PeekGangs(respool.PendingQueue, limit).Return([]*resmgrsvc.Gang{
		{
			Tasks: []*resmgr.Task{
				{
					Id: &peloton.TaskID{Value: "pendingqueue-job"},
				},
			},
		},
	}, nil).Times(1)
	mr.EXPECT().PeekGangs(respool.NonPreemptibleQueue, limit).Return([]*resmgrsvc.Gang{
		{
			Tasks: []*resmgr.Task{
				{
					Id: &peloton.TaskID{Value: "npqueue-job"},
				},
			},
		},
	}, nil).Times(1)
	mr.EXPECT().PeekGangs(respool.ControllerQueue, limit).Return([]*resmgrsvc.Gang{
		{
			Tasks: []*resmgr.Task{
				{
					Id: &peloton.TaskID{Value: "controllerqueue-job"},
				},
			},
		},
	}, nil).Times(1)
	mr.EXPECT().PeekGangs(respool.RevocableQueue, limit).Return([]*resmgrsvc.Gang{
		{
			Tasks: []*resmgr.Task{
				{
					Id: &peloton.TaskID{Value: "revocablequeue-job"},
				},
			},
		},
	}, nil).Times(1)

	mt := rm.NewMockTree(s.ctrl)
	mt.EXPECT().Get(respoolID).Return(mr, nil).Times(1)

	req := &resmgrsvc.GetPendingTasksRequest{
		RespoolID: respoolID,
		Limit:     limit,
	}

	handler := &ServiceHandler{
		metrics:     NewMetrics(tally.NoopScope),
		resPoolTree: mt,
		placements: queue.NewQueue(
			"placement-queue",
			reflect.TypeOf(resmgr.Placement{}),
			maxPlacementQueueSize,
		),
		rmTracker: s.rmTaskTracker,
		config: Config{
			RmTaskConfig: tasktestutil.CreateTaskConfig(),
		},
	}

	resp, err := handler.GetPendingTasks(s.context, req)
	s.NoError(err)

	s.Equal(4, len(resp.GetPendingGangsByQueue()))
	for q, gangs := range resp.GetPendingGangsByQueue() {
		s.Equal(1, len(gangs.GetPendingGangs()))
		expectedTaskID := ""
		switch q {
		case "controller":
			expectedTaskID = "controllerqueue-job"
		case "non-preemptible":
			expectedTaskID = "npqueue-job"
		case "pending":
			expectedTaskID = "pendingqueue-job"
		case "revocable":
			expectedTaskID = "revocablequeue-job"
		}

		for _, gang := range gangs.GetPendingGangs() {
			for _, tid := range gang.GetTaskIDs() {
				s.Equal(expectedTaskID, tid)
			}
		}
	}
}
