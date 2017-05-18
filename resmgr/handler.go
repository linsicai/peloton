package resmgr

import (
	"container/list"
	"context"
	"reflect"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber-go/tally"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/encoding/json"

	"code.uber.internal/infra/peloton/common"
	"code.uber.internal/infra/peloton/common/eventstream"
	"code.uber.internal/infra/peloton/common/queue"
	"code.uber.internal/infra/peloton/resmgr/respool"
	rmtask "code.uber.internal/infra/peloton/resmgr/task"

	t "code.uber.internal/infra/peloton/.gen/peloton/api/task"
	"code.uber.internal/infra/peloton/.gen/peloton/private/resmgr"
	"code.uber.internal/infra/peloton/.gen/peloton/private/resmgrsvc"
)

// serviceHandler implements peloton.private.resmgr.ResourceManagerService
// TODO: add placing and placed task queues
type serviceHandler struct {
	metrics            *Metrics
	resPoolTree        respool.Tree
	placements         queue.Queue
	eventStreamHandler *eventstream.Handler
	rmTracker          rmtask.Tracker
}

// InitServiceHandler initializes the handler for ResourceManagerService
func InitServiceHandler(
	d yarpc.Dispatcher,
	parent tally.Scope,
	rmTracker rmtask.Tracker) {

	handler := serviceHandler{
		metrics:     NewMetrics(parent.SubScope("resmgr")),
		resPoolTree: respool.GetTree(),
		placements: queue.NewQueue(
			"placement-queue",
			reflect.TypeOf(resmgr.Placement{}),
			maxPlacementQueueSize,
		),
		rmTracker: rmTracker,
	}
	// TODO: move eventStreamHandler buffer size into config
	handler.eventStreamHandler = initEventStreamHandler(d, 1000, parent.SubScope("resmgr"))

	json.Register(d, json.Procedure("ResourceManagerService.EnqueueTasks", handler.EnqueueTasks))
	json.Register(d, json.Procedure("ResourceManagerService.DequeueTasks", handler.DequeueTasks))
	json.Register(d, json.Procedure("ResourceManagerService.SetPlacements", handler.SetPlacements))
	json.Register(d, json.Procedure("ResourceManagerService.GetPlacements", handler.GetPlacements))
}

func initEventStreamHandler(d yarpc.Dispatcher, bufferSize int, parentScope tally.Scope) *eventstream.Handler {
	eventStreamHandler := eventstream.NewEventStreamHandler(
		bufferSize,
		[]string{
			common.PelotonJobManager,
			common.PelotonResourceManager,
		},
		nil,
		parentScope)
	json.Register(d, json.Procedure("EventStream.InitStream",
		eventStreamHandler.InitStream))
	json.Register(d, json.Procedure("EventStream.WaitForEvents",
		eventStreamHandler.WaitForEvents))
	return eventStreamHandler
}

// EnqueueTasks implements ResourceManagerService.EnqueueTasks
func (h *serviceHandler) EnqueueTasks(
	ctx context.Context,
	reqMeta yarpc.ReqMeta,
	req *resmgrsvc.EnqueueTasksRequest,
) (*resmgrsvc.EnqueueTasksResponse, yarpc.ResMeta, error) {

	log.WithField("request", req).Info("EnqueueTasks called.")
	h.metrics.APIEnqueueTasks.Inc(1)

	// Lookup respool from the resource pool tree
	respoolID := req.GetResPool()
	respool, err := respool.GetTree().Get(respoolID)
	if err != nil {
		h.metrics.EnqueueTaskFail.Inc(1)
		return &resmgrsvc.EnqueueTasksResponse{
			Error: &resmgrsvc.EnqueueTasksResponse_Error{
				NotFound: &resmgrsvc.ResourcePoolNotFound{
					Id:      respoolID,
					Message: err.Error(),
				},
			},
		}, nil, nil
	}
	// TODO: check if the user has permission to run tasks in the
	// respool

	// Enqueue the tasks sent in an API call to the pending queue of the respool.
	// First enqueue the tasks forming a gang and then the non-gang (singleton) tasks.
	// Note: jobmgr currently makes exactly 1 API call for each job; if jobmgr were
	// to instead combine jobs in one API call, the code should change to form separate
	// gangs per job and to retain job order for enqueueing, i.e., it should enqueue
	// job1-gang, job1-singletons, job2-gang, job2-singletons
	var failed []*resmgrsvc.EnqueueTasksFailure_FailedTask
	gangTasks := new(list.List)
	singletonTasks := new(list.List)
	for _, task := range req.GetTasks() {
		// Adding task to state machine
		err := h.rmTracker.AddTask(
			task,
			h.eventStreamHandler,
			respool,
		)
		if err != nil {
			failed = append(
				failed,
				&resmgrsvc.EnqueueTasksFailure_FailedTask{
					Task:    task,
					Message: err.Error(),
				},
			)
			h.metrics.EnqueueTaskFail.Inc(1)
			continue
		}
		if h.rmTracker.GetTask(task.Id) != nil {
			err = h.rmTracker.GetTask(task.Id).TransitTo(
				t.TaskState_PENDING.String())
			if err != nil {
				log.Error(err)
			}
		}
		if task.MinInstances > 1 {
			gangTasks.PushBack(task)
		} else {
			singletonTasks.PushBack(task)
		}
	}

	if gangTasks.Len() > 0 {
		err = respool.EnqueueSchedulingUnit(gangTasks) // enqueue gang
		// Report success/failure for all gang tasks
		for gangTask := gangTasks.Front(); gangTask != nil; gangTask = gangTask.Next() {
			if err != nil {
				failed = append(
					failed,
					&resmgrsvc.EnqueueTasksFailure_FailedTask{
						Task:    gangTask.Value.(*resmgr.Task),
						Message: err.Error(),
					},
				)
				h.metrics.EnqueueTaskFail.Inc(1)
			} else {
				h.metrics.EnqueueTaskSuccess.Inc(1)
			}
		}
	}
	if singletonTasks.Len() > 0 {
		for singletonItem := singletonTasks.Front(); singletonItem != nil; singletonItem = singletonItem.Next() {
			singletonTask := singletonItem.Value.(*resmgr.Task)
			err = respool.EnqueueSchedulingUnit(respool.MakeTaskSchedulingUnit(singletonTask))
			if err != nil {
				failed = append(
					failed,
					&resmgrsvc.EnqueueTasksFailure_FailedTask{
						Task:    singletonTask,
						Message: err.Error(),
					},
				)
				h.metrics.EnqueueTaskFail.Inc(1)
			} else {
				h.metrics.EnqueueTaskSuccess.Inc(1)
			}
		}
	}

	if len(failed) > 0 {
		return &resmgrsvc.EnqueueTasksResponse{
			Error: &resmgrsvc.EnqueueTasksResponse_Error{
				Failure: &resmgrsvc.EnqueueTasksFailure{
					Failed: failed,
				},
			},
		}, nil, nil
	}

	response := resmgrsvc.EnqueueTasksResponse{}
	log.Debug("Enqueue Returned")
	return &response, nil, nil
}

// DequeueTasks implements ResourceManagerService.DequeueTasks
func (h *serviceHandler) DequeueTasks(
	ctx context.Context,
	reqMeta yarpc.ReqMeta,
	req *resmgrsvc.DequeueTasksRequest,
) (*resmgrsvc.DequeueTasksResponse, yarpc.ResMeta, error) {

	h.metrics.APIDequeueTasks.Inc(1)

	limit := req.GetLimit()
	timeout := time.Duration(req.GetTimeout())
	readyQueue := rmtask.GetScheduler().GetReadyQueue()

	var tasks []*resmgr.Task
	for i := uint32(0); i < limit; i++ {
		item, err := readyQueue.Dequeue(timeout * time.Millisecond)
		if err != nil {
			log.Debug("Timeout to dequeue task from ready queue")
			h.metrics.DequeueTaskTimeout.Inc(1)
			break
		}
		task := item.(*resmgr.Task)
		tasks = append(tasks, task)
		h.metrics.DequeueTaskSuccess.Inc(1)

		// Moving task to Placing state
		err = h.rmTracker.GetTask(task.Id).TransitTo(
			t.TaskState_PLACING.String())
		if err != nil {
			log.WithError(err).WithField("taskID", task.Id.Value).
				Error("Failed to transit state for task")
		}
	}
	// TODO: handle the dequeue errors better
	response := resmgrsvc.DequeueTasksResponse{Tasks: tasks}
	log.WithField("response", response).Debug("DequeueTasks succeeded")
	return &response, nil, nil
}

// SetPlacements implements ResourceManagerService.SetPlacements
func (h *serviceHandler) SetPlacements(
	ctx context.Context,
	reqMeta yarpc.ReqMeta,
	req *resmgrsvc.SetPlacementsRequest,
) (*resmgrsvc.SetPlacementsResponse, yarpc.ResMeta, error) {

	log.WithField("request", req).Debug("SetPlacements called.")
	h.metrics.APISetPlacements.Inc(1)

	var failed []*resmgrsvc.SetPlacementsFailure_FailedPlacement
	var err error
	for _, placement := range req.GetPlacements() {
		err = h.placements.Enqueue(placement)
		if err != nil {
			log.WithField("placement", placement).
				WithError(err).Error("Failed to enqueue placement")
			failed = append(
				failed,
				&resmgrsvc.SetPlacementsFailure_FailedPlacement{
					Placement: placement,
					Message:   err.Error(),
				},
			)
			h.metrics.SetPlacementFail.Inc(1)
		} else {
			h.metrics.SetPlacementSuccess.Inc(1)
			// Transitioning tasks from Placing to Placed
			for _, taskID := range placement.Tasks {
				if h.rmTracker.GetTask(taskID) != nil {
					err := h.rmTracker.GetTask(taskID).
						TransitTo(t.TaskState_PLACED.String())
					if err != nil {
						log.WithError(err).Error("Not able " +
							"to transition to placed " +
							"for task " + taskID.Value)
					}
				}
			}
		}
	}
	if len(failed) > 0 {
		return &resmgrsvc.SetPlacementsResponse{
			Error: &resmgrsvc.SetPlacementsResponse_Error{
				Failure: &resmgrsvc.SetPlacementsFailure{
					Failed: failed,
				},
			},
		}, nil, nil
	}
	response := resmgrsvc.SetPlacementsResponse{}
	h.metrics.PlacementQueueLen.Update(float64(h.placements.Length()))
	log.Debug("Set Placement Returned")
	return &response, nil, nil
}

// GetPlacements implements ResourceManagerService.GetPlacements
func (h *serviceHandler) GetPlacements(
	ctx context.Context,
	reqMeta yarpc.ReqMeta,
	req *resmgrsvc.GetPlacementsRequest,
) (*resmgrsvc.GetPlacementsResponse, yarpc.ResMeta, error) {

	log.WithField("request", req).Debug("GetPlacements called.")
	h.metrics.APIGetPlacements.Inc(1)

	limit := req.GetLimit()
	timeout := time.Duration(req.GetTimeout())

	h.metrics.APIGetPlacements.Inc(1)
	var placements []*resmgr.Placement
	for i := 0; i < int(limit); i++ {
		item, err := h.placements.Dequeue(timeout * time.Millisecond)

		if err != nil {
			h.metrics.GetPlacementFail.Inc(1)
			break
		}
		placement := item.(*resmgr.Placement)
		placements = append(placements, placement)
		h.metrics.GetPlacementSuccess.Inc(1)
	}
	response := resmgrsvc.GetPlacementsResponse{Placements: placements}
	h.metrics.PlacementQueueLen.Update(float64(h.placements.Length()))
	log.Debug("Get Placement Returned")

	return &response, nil, nil
}
