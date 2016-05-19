package main

import (
	"flag"
	"fmt"

	"github.com/mesos/mesos-go/executor"
	"github.com/mesos/mesos-go/mesosproto"
	"net/http"

	log "github.com/Sirupsen/logrus"
)

type exampleExecutor struct {
	timesLaunched int
}

func newExampleExecutor() *exampleExecutor {
	return &exampleExecutor{timesLaunched: 0}
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

//LaunchTask is an implementation required by Mesos
func (e *exampleExecutor) LaunchTask(driver executor.ExecutorDriver, taskInfo *mesosproto.TaskInfo) {
	fmt.Printf("Launching task %v with data [%#x]\n", taskInfo.GetName(), taskInfo.Data)

	runStatus := &mesosproto.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesosproto.TaskState_TASK_RUNNING.Enum(),
	}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		fmt.Println("Got error", err)
	}

	e.timesLaunched++

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		w.Write(taskInfo.Data)
	})

	log.Info("Running server in port 8080")
	http.ListenAndServe(":8080", nil)

	// Finish task
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

func (exec *exampleExecutor) KillTask(executor.ExecutorDriver, *mesosproto.TaskID) {
	fmt.Println("Kill task")
}

func (exec *exampleExecutor) FrameworkMessage(driver executor.ExecutorDriver, msg string) {
	fmt.Println("Got framework message: ", msg)
}

func (exec *exampleExecutor) Shutdown(executor.ExecutorDriver) {
	fmt.Println("Shutting down the executor")
}

func (exec *exampleExecutor) Error(driver executor.ExecutorDriver, err string) {
	fmt.Println("Got error message:", err)
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
