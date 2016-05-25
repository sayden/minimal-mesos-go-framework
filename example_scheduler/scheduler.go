package example_scheduler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"github.com/satori/go.uuid"
)

type ExampleScheduler struct {
	ExecutorInfo *mesosproto.ExecutorInfo

	//The CPUs that the tasks need
	NeededCpu float64

	//The RAM that the tasks need
	NeededRam float64

	launched bool
}

//StatusUpdate is called by a running task to provide status information to the
//scheduler.
func (s *ExampleScheduler) StatusUpdate(driver scheduler.SchedulerDriver, status *mesosproto.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())

	if status.GetState() == mesosproto.TaskState_TASK_RUNNING {
		s.launched = true
		log.Info("Server is running")
	}

	if status.GetState() == mesosproto.TaskState_TASK_FINISHED {
		log.Info("Server is finished")
	}

	if status.GetState() == mesosproto.TaskState_TASK_LOST ||
		status.GetState() == mesosproto.TaskState_TASK_KILLED ||
		status.GetState() == mesosproto.TaskState_TASK_FAILED {
		log.Infoln(
			"Aborting because task", status.TaskId.GetValue(),
			"is in unexpected state", status.State.String(),
			"with message: ", status.GetMessage(),
		)
		driver.Abort()
	}
}

//ResourceOffers will be called by the Mesos framework to provide an array of
//offers to this framework. Is up to you to check the content of the offers
//and to accept or reject them if they don't fit the needs of the framework
func (s *ExampleScheduler) ResourceOffers(driver scheduler.SchedulerDriver, offers []*mesosproto.Offer) {
	for _, offer := range offers {
		if s.launched {
			driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
			continue
		}

		offeredCpu := 0.0
		offeredMem := 0.0
		var offeredPort []*mesosproto.Value_Range = make([]*mesosproto.Value_Range,1)

		for _, resource := range offer.Resources {
			if resource.GetName() == "cpus" {
				offeredCpu += resource.GetScalar().GetValue()
			} else if resource.GetName() == "mem" {
				offeredMem += resource.GetScalar().GetValue()
			} else if resource.GetName() == "ports" {
				log.Infof("Offered ports: %v", resource.GetRanges())
				ranges := resource.GetRanges()

				//Take the first value of the range as we only need one port
				if len(ranges.Range) > 0 {
					firstRange := ranges.Range[0]

					uniquePortRange := mesosproto.Value_Range{
						Begin:firstRange.Begin,
						End:firstRange.Begin,
					}

					offeredPort[0] = &uniquePortRange
				}
			}
		}

		//Print information about the received offer
		log.Infof("Received Offer <%v> with cpus=%v mem=%v, port=%v",
			offer.Id.GetValue(),
			offeredCpu,
			offeredMem,
			offeredPort)

		//Decline offer if the offer doesn't satisfy our needs
		if offeredCpu < s.NeededCpu || offeredMem < s.NeededRam || len(offeredPort) == 0 {
			log.Infof("Declining offer <%v>\n", offer.Id.GetValue())
			driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
			continue
		}

		// At this point we have determined we will be accepting at least part of this offer
		s.launched = true
		var tasks []*mesosproto.TaskInfo

		// We have to create a TaskID so we use the go-uuid library to create
		// a random id.
		taskId := &mesosproto.TaskID{
			Value: proto.String(uuid.NewV4().String()),
		}

		log.Infof("Offered portss: %v", offeredPort)

		//Provide information about the name of the task, id, the slave will
		//be run of, the executor (that contains the command to execute as well
		//as the uri to download the executor or executors from and the amount
		//of resource the taks will use (not neccesary all from the offer)
		task := &mesosproto.TaskInfo{
			Name:     proto.String("go-task-" + taskId.GetValue()),
			TaskId:   taskId,
			SlaveId:  offer.SlaveId,
			Executor: s.ExecutorInfo,
			Resources: []*mesosproto.Resource{
				mesosutil.NewScalarResource("cpus", s.NeededCpu),
				mesosutil.NewScalarResource("mem", s.NeededRam),
				mesosutil.NewRangesResource("port", offeredPort),
			},
			Data: []byte("Hello from Server"),
		}

		log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), offer.Id.GetValue())

		tasks = append(tasks, task)

		log.Infoln("Launching task for offer", offer.Id.GetValue())

		//Launch the task
		driver.LaunchTasks([]*mesosproto.OfferID{offer.Id}, tasks, &mesosproto.Filters{RefuseSeconds: proto.Float64(10)})
	}
}
