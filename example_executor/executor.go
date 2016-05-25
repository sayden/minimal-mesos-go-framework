package main

import (
	"flag"
	"fmt"

	"net/http"

	"github.com/mesos/mesos-go/executor"
	"github.com/mesos/mesos-go/mesosproto"

	"io/ioutil"

	log "github.com/Sirupsen/logrus"
)

type exampleExecutor struct {
	timesLaunched int
}

func newExampleExecutor() *exampleExecutor {
	return &exampleExecutor{timesLaunched: 0}
}

//LaunchTask is an implementation required by Mesos
func (e *exampleExecutor) LaunchTask(driver executor.ExecutorDriver, taskInfo *mesosproto.TaskInfo) {
	fmt.Printf("Launching task %v with data [%#x]\n", taskInfo.GetName(), taskInfo.Data)
	logg("launching task")

	//Send a status update to the scheduler
	runStatus := &mesosproto.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesosproto.TaskState_TASK_RUNNING.Enum(),
	}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		fmt.Println("Got error", err)
		logg("got error 1")
	}

	e.timesLaunched++

	launchMyServer(taskInfo)

	//Send a status update to the scheduler
	fmt.Println("Finishing task", taskInfo.GetName())
	finStatus := &mesosproto.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesosproto.TaskState_TASK_FINISHED.Enum(),
	}
	_, err = driver.SendStatusUpdate(finStatus)
	if err != nil {
		fmt.Println("Got error", err)
		logg("got error 2")
		return
	}

	fmt.Println("Task finished", taskInfo.GetName())
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("Starting Example Executor (Go)")

	dconfig := executor.DriverConfig{
		Executor: newExampleExecutor(),
	}
	driver, err := executor.NewMesosExecutorDriver(dconfig)

	if err != nil {
		fmt.Println("Unable to create a ExecutorDriver ", err.Error())
	}

	_, err = driver.Start()
	if err != nil {
		fmt.Println("Got error:", err)
		return
	}
	fmt.Println("Executor process has started and running.")
	driver.Join()
}

//launchMyServer launched an blocking HTTP server with the data provided by the
//scheduler
func launchMyServer(taskInfo *mesosproto.TaskInfo) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(taskInfo.Data)
	})

	var port int

	for _, resource := range taskInfo.Resources {
		if resource.GetName() == "ports" {
			port = int(resource.GetRanges().GetRange()[0].GetBegin())
		}
	}

	log.Info("Running server in port 54231")
	http.ListenAndServe(":" + string(port), nil)
}

func logg(msg string) {
	ioutil.WriteFile("/tmp/test", []byte(msg), 0644)
}

//Registered is an implementation required by Mesos
func (e *exampleExecutor) Registered(driver executor.ExecutorDriver, execInfo *mesosproto.ExecutorInfo, fwinfo *mesosproto.FrameworkInfo, slaveInfo *mesosproto.SlaveInfo) {
	fmt.Println("Registered Executor on slave ", slaveInfo.GetHostname())
	logg("registered")
}

//Reregistered is an implementation required by Mesos
func (e *exampleExecutor) Reregistered(driver executor.ExecutorDriver, slaveInfo *mesosproto.SlaveInfo) {
	fmt.Println("Re-registered Executor on slave ", slaveInfo.GetHostname())
	logg("re registered")
}

//Disconnected is an implementation required by Mesos
func (e *exampleExecutor) Disconnected(executor.ExecutorDriver) {
	fmt.Println("Executor disconnected.")
	logg("Executor disconnected.")
}

func (e *exampleExecutor) KillTask(executor.ExecutorDriver, *mesosproto.TaskID) {
	fmt.Println("Kill task")
	logg("kill task")
}

func (e *exampleExecutor) FrameworkMessage(driver executor.ExecutorDriver, msg string) {
	fmt.Println("Got framework message: ", msg)
	logg("got framework message")
}

func (e *exampleExecutor) Shutdown(executor.ExecutorDriver) {
	fmt.Println("Shutting down the executor")
	logg("shutting down")
}

func (e *exampleExecutor) Error(driver executor.ExecutorDriver, err string) {
	fmt.Println("Got error message:", err)
	logg("got error message")
}
