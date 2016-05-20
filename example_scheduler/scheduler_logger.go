package example_scheduler

import (
	log "github.com/Sirupsen/logrus"
	"github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/scheduler"
)

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
