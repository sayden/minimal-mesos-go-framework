package main

import (
	"flag"
	"fmt"

	"net/http"

	"github.com/mesos/mesos-go/executor"
	"github.com/mesos/mesos-go/mesosproto"

	"strconv"

	log "github.com/Sirupsen/logrus"
)

type exampleExecutor struct {}

//LaunchTask is an implementation required by Mesos
func (e *exampleExecutor) LaunchTask(driver executor.ExecutorDriver, taskInfo *mesosproto.TaskInfo) {
	fmt.Printf("Launching task %v with data [%#x]\n", taskInfo.GetName(), taskInfo.Data)

	var port string

	for _, resource := range taskInfo.Resources {
		if resource.GetName() == "ports" {
			port = strconv.FormatUint(resource.GetRanges().GetRange()[0].GetBegin(), 10)
		}
	}

	//Send a status update to the scheduler
	runStatus := &mesosproto.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesosproto.TaskState_TASK_RUNNING.Enum(),
	}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		fmt.Println("Got error", err)
	}

	launchMyServer(taskInfo.Data, port)

	//Send a status update to the scheduler
	fmt.Println("Finishing task", taskInfo.GetName())
	finStatus := &mesosproto.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesosproto.TaskState_TASK_FINISHED.Enum(),
	}
	_, err = driver.SendStatusUpdate(finStatus)
	if err != nil {
		fmt.Println("Got error", err)
		return
	}

	fmt.Println("Task finished", taskInfo.GetName())
}

func init() {
	flag.Parse()
}

func main() {
	fmt.Println("Starting Example Executor (Go)")

	driverConfig := executor.DriverConfig{
		Executor: &exampleExecutor{},
	}
	driver, err := executor.NewMesosExecutorDriver(driverConfig)

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
func launchMyServer(data []byte, port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})

	log.Infof("Running server in port %s", port)
	http.ListenAndServe(":"+port, nil)
}

//Registered is an implementation required by Mesos
func (e *exampleExecutor) Registered(driver executor.ExecutorDriver, execInfo *mesosproto.ExecutorInfo, fwinfo *mesosproto.FrameworkInfo, slaveInfo *mesosproto.SlaveInfo) {
	fmt.Println("Registered Executor on slave ", slaveInfo.GetHostname())
}

//Reregistered is an implementation required by Mesos
func (e *exampleExecutor) Reregistered(driver executor.ExecutorDriver, slaveInfo *mesosproto.SlaveInfo) {
	fmt.Println("Re-registered Executor on slave ", slaveInfo.GetHostname())
}

//Disconnected is an implementation required by Mesos
func (e *exampleExecutor) Disconnected(executor.ExecutorDriver) {
	fmt.Println("Executor disconnected.")
}

func (e *exampleExecutor) KillTask(executor.ExecutorDriver, *mesosproto.TaskID) {
	fmt.Println("Kill task")
}

func (e *exampleExecutor) FrameworkMessage(driver executor.ExecutorDriver, msg string) {
	fmt.Println("Got framework message: ", msg)
}

func (e *exampleExecutor) Shutdown(executor.ExecutorDriver) {
	fmt.Println("Shutting down the executor")
}

func (e *exampleExecutor) Error(driver executor.ExecutorDriver, err string) {
	fmt.Println("Got error message:", err)
}
