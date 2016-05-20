package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"github.com/satori/go.uuid"
)

type ExampleScheduler struct {
	executorInfo  *mesosproto.ExecutorInfo
	neededCpu     float64
	neededRam     float64
	timesLaunched int
	launched      bool
}

//NewExampleScheduler returns a Scheduler filled with the provided information
func NewExampleScheduler(e *mesosproto.ExecutorInfo, cpu float64, ram float64) *ExampleScheduler {
	return &ExampleScheduler{
		executorInfo: e,
		neededCpu:    cpu,
		neededRam:    ram,
	}
}

func (s *ExampleScheduler) Registered(driver scheduler.SchedulerDriver, frameworkId *mesosproto.FrameworkID, masterInfo *mesosproto.MasterInfo) {
	log.Infoln("Scheduler Registered with Master ", masterInfo)
}

func (s *ExampleScheduler) Reregistered(driver scheduler.SchedulerDriver, masterInfo *mesosproto.MasterInfo) {
	log.Infoln("Scheduler Re-Registered with Master ", masterInfo)
}

func (s *ExampleScheduler) Disconnected(scheduler.SchedulerDriver) {
	log.Infoln("Scheduler Disconnected")
}

func (sched *ExampleScheduler) OfferRescinded(s scheduler.SchedulerDriver, id *mesosproto.OfferID) {
	log.Infof("Offer '%v' rescinded.\n", *id)
}

func (sched *ExampleScheduler) FrameworkMessage(s scheduler.SchedulerDriver, exId *mesosproto.ExecutorID, slvId *mesosproto.SlaveID, msg string) {
	log.Infof("Received framework message from executor '%v' on slave '%v': %s.\n", *exId, *slvId, msg)
}

func (sched *ExampleScheduler) SlaveLost(s scheduler.SchedulerDriver, id *mesosproto.SlaveID) {
	log.Infof("Slave '%v' lost.\n", *id)
}

func (sched *ExampleScheduler) ExecutorLost(s scheduler.SchedulerDriver, exId *mesosproto.ExecutorID, slvId *mesosproto.SlaveID, i int) {
	log.Infof("Executor '%v' lost on slave '%v' with exit code: %v.\n", exId.GetValue(), slvId.GetValue(), i)
}

func (sched *ExampleScheduler) Error(driver scheduler.SchedulerDriver, err string) {
	log.Infoln("Scheduler received error:", err)
}

func (s *ExampleScheduler) StatusUpdate(driver scheduler.SchedulerDriver, status *mesosproto.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())

	if status.GetState() == mesosproto.TaskState_TASK_RUNNING {
		s.timesLaunched++
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

func (s *ExampleScheduler) ResourceOffers(driver scheduler.SchedulerDriver, offers []*mesosproto.Offer) {
	for _, offer := range offers {
		if s.launched {
			driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
			continue
		}
		log.Infof("Received Offer <%v> with cpus=%v mem=%v", offer.Id.GetValue(), getOfferCpu(offer), getOfferMem(offer))

		offeredCpu := getOfferCpu(offer)
		offeredMem := getOfferMem(offer)

		//Decline offer if the offer doesn't satisfy our needs
		if offeredCpu < s.neededCpu || offeredMem < s.neededRam {
			log.Infof("Declining offer <%v>\n", offer.Id.GetValue())
			driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
			continue
		}

		// At this point we have determined we will be accepting at least part of this offer
		s.launched = true
		var tasks []*mesosproto.TaskInfo

		taskId := &mesosproto.TaskID{
			Value: proto.String(uuid.NewV4().String()),
		}

		task := &mesosproto.TaskInfo{
			Name:     proto.String("go-task-" + taskId.GetValue()),
			TaskId:   taskId,
			SlaveId:  offer.SlaveId,
			Executor: s.executorInfo,
			Resources: []*mesosproto.Resource{
				mesosutil.NewScalarResource("cpus", s.neededCpu),
				mesosutil.NewScalarResource("mem", s.neededRam),
			},
			Data: []byte("Hello from Server"),
		}

		log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), offer.Id.GetValue())

		tasks = append(tasks, task)

		log.Infoln("Launching task for offer", offer.Id.GetValue())
		driver.LaunchTasks([]*mesosproto.OfferID{offer.Id}, tasks, &mesosproto.Filters{RefuseSeconds: proto.Float64(10)})
	}
}

func getOfferScalar(offer *mesosproto.Offer, name string) float64 {
	resources := mesosutil.FilterResources(offer.Resources, func(res *mesosproto.Resource) bool {
		return res.GetName() == name
	})

	value := 0.0
	for _, res := range resources {
		value += res.GetScalar().GetValue()
	}

	return value
}

func getOfferCpu(offer *mesosproto.Offer) float64 {
	return getOfferScalar(offer, "cpus")
}

func getOfferMem(offer *mesosproto.Offer) float64 {
	return getOfferScalar(offer, "mem")
}
