package task

import (
	"sync"

	"code.uber.internal/go-common.git/x/log"
	master_mesos "code.uber.internal/infra/peloton/master/mesos"
	"code.uber.internal/infra/peloton/util"
	"code.uber.internal/infra/peloton/yarpc/encoding/mpb"
	"go.uber.org/yarpc"

	mesos "mesos/v1"
	sched "mesos/v1/scheduler"
	"peloton/task"
)

// Launcher is the interface to launch a set of tasks using an offer
type Launcher interface {
	LaunchTasks(offer *mesos.Offer, tasks []*task.TaskInfo) error
}

type taskLauncher struct {
	client mpb.Client
}

var instance *taskLauncher
var once sync.Once

// GetTaskLauncher returns the task launcher
func GetTaskLauncher(d yarpc.Dispatcher, mesosClient mpb.Client) Launcher {
	once.Do(func() {
		instance = &taskLauncher{
			client: mesosClient,
		}
	})
	return instance
}

// LaunchTasks launches a list of tasks using an offer
func (t *taskLauncher) LaunchTasks(
	offer *mesos.Offer, tasks []*task.TaskInfo) error {
	var mesosTasks []*mesos.TaskInfo
	var mesosTaskIds []string
	for _, t := range tasks {
		mesosTask := util.ConvertToMesosTaskInfo(t)
		mesosTask.AgentId = offer.AgentId
		mesosTasks = append(mesosTasks, mesosTask)
		mesosTaskIds = append(mesosTaskIds, *mesosTask.TaskId.Value)
	}
	callType := sched.Call_ACCEPT
	opType := mesos.Offer_Operation_LAUNCH
	msg := &sched.Call{
		FrameworkId: offer.FrameworkId,
		Type:        &callType,
		Accept: &sched.Call_Accept{
			OfferIds: []*mesos.OfferID{offer.Id},
			Operations: []*mesos.Offer_Operation{
				{
					Type: &opType,
					Launch: &mesos.Offer_Operation_Launch{
						TaskInfos: mesosTasks,
					},
				},
			},
		},
	}
	// TODO: add retry / put back offer and tasks in failure scenarios
	msid := master_mesos.GetSchedulerDriver().GetMesosStreamID()
	err := t.client.Call(msid, msg)
	if err != nil {
		log.Warnf("Failed to launch %v tasks using offer %v, err=%v",
			len(tasks), *offer.GetId().Value, err)
	} else {
		log.Debugf("Launched %v tasks %v using offer %v",
			len(tasks), mesosTaskIds, *offer.GetId().Value)
	}
	return err
}
