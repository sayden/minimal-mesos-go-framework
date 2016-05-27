# minimal-mesos-go-framework

Minimally possible Mesos framework that launches a long running task (a web server). It could be even smaller if we run a command line command, but we wanted to have something closer to what Mesos could be used for.


## How to create a Mesos framework in Go

The process involved in create a Mesos framework is divided in two steps: create a Mesos Scheduler and a Mesos Executor.


## 1 Creating a Mesos Scheduler
The Mesos Scheduler is the piece that will talk with the Mesos cluster and decide if it should accept an offer of the cluster, reject it or partially accept it. This is done manually by iterating through the offers, checking their contents and comparing them with the needed resources by your framework or job.

If your framework needs to launch more than one instance (for example: let's say you want to launch a cluster of MongoDB with 1 master and 3 slaves), you have to manually allocate the resources (accept the offers) for the machines and make the appropriate initialization from the scheduler to launch first the Master, then launch the slaves and connect them to the Master instance.

So, let's try to create the smallest possible Scheduler in Go. First of all, download the dependencies:

```bash
go get -u github.com/golang/protobuf
go get -u github.com/mesos/mesos-go
```

Protobuf is used to communicate with Mesos (which is developed in C++) from other languages (like Go, Java...) using RPC. mesos-go is Go's library to use Mesos. To write a minimal scheduler, we'll need few things:

* **(1)** *Uris for the executors*: Is a struct called "mesosproto.CommandInfo_URI" that we'll fill with the URI where the cluster can download/fetch our executor (that we've previously uploaded somewhere, more about this later).
* **(2)** *Information about the Executor*: Represented by the "mesosproto.ExecutorInfo" struct where we'll set an ID for the executor, a name, a source (a prefix identifier to track the "sources" of an executor so that you can relate different executors to the same scheduler) and a command (to launch, somehow, the executor).
* **(3)** *An implementation of the "scheduler.SchedulerDriver" interface*, found in package mesos-go in a custom struct. On this implementation you'll have to define what you want to do when you receive an offer or a status update from a executor.
* **(4)** *Information about the Framework*: A user on the cluster to run executors and a name for the framework.
* **(5)** *Credential information*: If the cluster is securized. In out example we set it to "nil" as we won't implement security to keep things easy.
* **(6)** *Cluster information*: Master's IP address and port.
* **(7)** *A Scheduler driver*: This is "simply" a struct to compose all the previous information.

To recall:
- (7) Scheduler driver:
	- (3) Custom Scheduler:
		- (2) Executor Info:
			- Executor ID:
			- Name
			- Source
			- Command
				- Value
				- (1) Uris
	- (4) Framework Info:
		- User
		- Framework name
	- (6) Master IP:Port
	- (5) Credential information

### Filling Executor information

```go
// URIs
executorUri := "http://s3-eu-west-1.amazonaws.com/enablers/executor"
executorUris := []*mesosproto.CommandInfo_URI{
   {
      Value:      &executorUri,
      Executable: proto.Bool(true),
   },
}

//Info
executorInfo := &mesosproto.ExecutorInfo{
   ExecutorId: mesosutil.NewExecutorID("default"),
   Name:       proto.String("Test Executor (Go)"),
   Source:     proto.String("go_test"),
   Command: &mesosproto.CommandInfo{
      Value: proto.String("./executor"),
      Uris:  executorUris,
   },
}

//Scheduler
my_scheduler := &example_scheduler.ExampleScheduler{
	ExecutorInfo: executorInfo,
	NeededCpu:    0.5,
	NeededRam:    128.0,
}
```

Here, we'll set the URL where the cluster can download the executor in the variable "executorUri" and fill a CommandInfo_URI struct. We also set that the downloaded file is an executable (you could also set it as a compressed file so that it extracts it into the sandbox). Once we have the URI's, we created an ExecutorInfo to set the information we mention above.

Next, we'll create an instance of our ExampleScheduler (we haven't created it yet) where we'll set the CPU and Memory that the framework needs and the ExecutorInfo instance that we've just created (needed for the task later).

### Filling Framework information
```go

//Framework
frameworkInfo := &mesosproto.FrameworkInfo{
   User: proto.String("root"), // Mesos-go will fill in user.
   Name: proto.String("Web Server Framework (Go)"),
}
```

### Creating an Example Scheduler
Now we'll create a struct that will implement "SchedulerDriver" interface:

```go
type ExampleScheduler struct {
	ExecutorInfo *mesosproto.ExecutorInfo

	//The CPUs that the tasks need
	NeededCpu float64

	//The RAM that the tasks need
	NeededRam float64

	launched bool
}
```

We will implement extensively one method (`ResourceOffers`) and the rest simply do some logging.
* `StatusUpdate`: This method will be called when the Executor notify scheduler with some update on their status. We'll see later that this status update and information must be sent manually when implementing the Executor. In our example we don't do anything special, We simply setup some logging depending on the status received. Pay attention that we abort the driver when a "Lost", "Killed" or "Failed" status is received.
* `ResourceOffers`: ResourceOffers will be called on every offer from the cluster, even when executor are already running and is up to you to refuse offers once you have launched all executors you need. Let's analyze it step by step:

#### Receiving, accepting and declining offers

First of all, we'll iterate through all offers received (because **you'll receive offers in an array, not one by one**). Then, we checked that our example task isn't running to decline this offer if it is (and continue to next). You could be thinking why I don't check if the executor is running before iterating the offers, this is to actively refuse the offers that are received so that they're available as soon as possible to other frameworks.

```go
func (s *ExampleScheduler) ResourceOffers(driver scheduler.SchedulerDriver, offers []*mesosproto.Offer) {
   for _, offer := range offers {
      if s.launched {
         driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
         continue
      }
...

```

Next, we checked the contents of the offer. In the offer, each resource is of one type (cpu, mem, port...) defined by the field "Name" and recovered using "GetName()"

```go
func (s *ExampleScheduler) ResourceOffers(driver scheduler.SchedulerDriver, offers []*mesosproto.Offer) {
    for _, offer := range offers {
        if s.launched {
            driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
            continue
        }


        		offeredCpu := 0.0
        		offeredMem := 0.0
        		var offeredPort []*mesosproto.Value_Range = make([]*mesosproto.Value_Range, 1)

        		// Iterate over resources to find one that fits all our needs. This
        		// usually isn't done this way as you must accept an offer that cover
        		// partially your needs and keep accepting until you fit all of them
        		for _, resource := range offer.Resources {
        			switch resource.GetName() {
        			case "cpus":
        				offeredCpu += resource.GetScalar().GetValue()
        			case "mem":
        				offeredMem += resource.GetScalar().GetValue()
        			case "ports":
        				ranges := resource.GetRanges()

        				//Take the first value of the range as we only need one port
        				if len(ranges.Range) > 0 {
        					firstRange := ranges.Range[0]

        					uniquePortRange := mesosproto.Value_Range{
        						Begin: firstRange.Begin,
        						End:   firstRange.Begin,
        					}

        					offeredPort[0] = &uniquePortRange
        				}
        			}
        		}

		//Print information about the received offer
		log.Infof("Received Offer <%v> with cpus=%v mem=%v, port=%v from %s",
			offer.Id.GetValue(),
			offeredCpu,
			offeredMem,
			offeredPort[0].GetBegin(),
			*offer.Hostname)
...
...

```

After checking if our unique job is already launched, we create 3 variables that will hold the values of the offers so that we can check if they fit our needs later. We iterate over each resource using a switch to check the offered resource. As you can see, ports are slightly complex because their offers comes in ranges (of 100 ports, for example) so you have to create a new offer just with the ones you need (maybe one, maybe all of them).

```go
	//Decline offer if the offer doesn't satisfy our needs
	if offeredCpu < s.NeededCpu || offeredMem < s.NeededRam || len(offeredPort) == 0 {
		log.Infof("Declining offer <%v>\n", offer.Id.GetValue())
		driver.DeclineOffer(offer.Id, &mesosproto.Filters{RefuseSeconds: proto.Float64(1)})
		continue
	}
```

So now we checked if the contents of the offers fit the needs of our `ExampleScheduler` that represents the needs of the executor (you don't have to store the needs in the scheduler this way but we did it this way for simplicity). If the offer doesn't fit our needs we explicitly decline it and check the next offer.

```go
// At this point we have determined we accept the offer

// We have to create a TaskID so we use the go-uuid library to create
// a random id.
taskId := &mesosproto.TaskID{
  Value: proto.String(uuid.NewV4().String()),
}

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
  	mesosutil.NewRangesResource("ports", offeredPort),
  },
  Data: []byte("Hello from Server"),
}

log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), offer.Id.GetValue())
```

Once we have an offer that satisfy our needs we have to compose a `mesosproto.TaskInfo` with a `mesosproto.TaskID` and some information:
* A name for the task. Here we prefix it with `go-task-` and we've added the id we generated for the `TaskID` using a special library to generate uuids.
* The TaskID object that we've created just over it
* The executor information that we passed on ExampleScheduler creation
* The Resources needs. Not the offers that you've received but the specific needs of your framework (that most of the times will be less than the offer).
* Some Data that you want to pass to the executors.


```go
  var tasks []*mesosproto.TaskInfo
	tasks = append(tasks, task)

	log.Infoln("Launching task for offer", offer.Id.GetValue())

	//Launch the task
	status, err := driver.LaunchTasks([]*mesosproto.OfferID{offer.Id}, tasks, &mesosproto.Filters{RefuseSeconds: proto.Float64(10)})
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Launch task status: %v", status)
	s.launched = true
```

Finally, we have to create an array of TaskInfo object to accept the offer (as we can launch one or many tasks). We launch the tasks with `LaunchTasks` by passing an array of offers ID's (we can accept more than one offer to fit our needs) and we set the `launched` variable to `true` so we decline offers from this point.

#### Receiving status messages
We also should implement a method that help us to receive status messages from the executors:
```go
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
```

Here, we mainly do logging but, in case something went wrong, we abort the SchedulerDriver so that no more callbacks can be made to the scheduler.

#### Implementing the rest of the interface
To finish our `ExampleScheduler`, the interface we're using has many other methods that must be implemented. For simplicity, we'll simply make some logging with them.

```go
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
```

### Filling Framework information
To finish our Scheduler, we have to fill a `mesosproto.FrameworkInfo` object:

```go
//Framework
frameworkInfo := &mesosproto.FrameworkInfo{
	User: proto.String("root"), // Mesos-go will fill in user.
	Name: proto.String("Stratio Server Framework (Go)"),
}
```

Special consideration must be taken with the "User" field. This user can be filled by Mesos or by you in case you need to execute the Framework using a specific user in the cluster. This is important if, for example, you run a Framework that needs `root` access to achieve something.

### Creating the SchedulerDriver
```go


	//Scheduler Driver
	config := scheduler.DriverConfig{
		Scheduler:  my_scheduler,
		Framework:  frameworkInfo,
		Master:     *master,
		Credential: (*mesosproto.Credential)(nil),
	}

	driver, err := scheduler.NewMesosSchedulerDriver(config)

	if err != nil {
		log.Fatalf("Unable to create a SchedulerDriver: %v\n", err.Error())
		os.Exit(-3)
	}

	if stat, err := driver.Run(); err != nil {
		log.Fatalf("Framework stopped with status %s and error: %s\n", stat.String(), err.Error())
		os.Exit(-4)
	}
```

We fill a `scheduler.DriverConfig` object with every object we've created until now. We don't use authentication so we pass `nil` as credentials.

Once we use the "Builder" on method `NewMesosSchedulerDriver` we can run the driver using `driver.Run()` and capture the stat and the error in case they happen.

We have our Scheduler ready. Let's go with the Executor

## 2 Creating an Executor.
An Executor is simpler than a Scheduler. You must also implement an interface (the `Executor` interface) and all its methods. The most important method is `LaunchTask` that, as you have already guessed, launches whichever task you need on the cluster.
```go
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
```

Creating a struct to implement the `Executor` interface, the implementation of `LaunchTask` is simple. First we'll recover the port that the Scheduler has assign to me. As we mentioned before, ports offers comes in the form of ranges so we first iterate over all resources looking for the "ports" one and we get the first occurrence. We'll only need one port as this is what we accepted in the previous Scheduler.

We notify the Scheduler that we're running sending a `mesosproto.TaskStatus` object. This is incorrect as we haven't reached the line that launches the server but for the purpose of simplicity it's ok.

`launchMyServer` is a method that we haven't written yet but that simply launches an HTTP server showing the content we set on the `Data` field in the Scheduler. This method is blocking and the execution stops here so if the server crashes the code will continue and notify the Scheduler with the `TaskState_TASK_FINISHED` message. `launchMyServer` has the following content

```go
//launchMyServer launched an blocking HTTP server with the data provided by the
//scheduler
func launchMyServer(data []byte, port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	})

	log.Infof("Running server in port %s", port)
	http.ListenAndServe(":"+port, nil)
}
```

HTTP server. Can't be simpler.

On the executor we also need to implement the following method that, for simplicity they just prints some info.

```go
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

```

Finally, in our main, we create a `executor.DriverConfig` and we pass it to the Builder to get a Driver, start it and join the event loop to wait for the notification of driver termination.

## 3 Executing the Framework

Finally, our framework is ready and we can execute it. We have uploaded our Executor to a public URL in AWS so that it's available for any Mesos cluster.

```bash
$ ./scheduler --master zk://paas2:2181/mesos
2016/05/27 11:49:15 Connected to 10.200.0.153:2181
2016/05/27 11:49:15 Authenticated: id=95962866784272418, timeout=40000
INFO[0000] Scheduler Registered with Master  &MasterInfo{Id:*a10e0385-8988-4e02-a9c7-e3b8bba7bf95,Ip:*2566965258,Port:*5050,Pid:*master@10.200.0.153:5050,Hostname:*10.200.0.153,Version:*0.28.1,Address:&Address{Hostname:*10.200.0.153,Ip:*10.200.0.153,Port:*5050,XXX_unrecognized:[],},XXX_unrecognized:[],} 
INFO[0000] Received Offer <a10e0385-8988-4e02-a9c7-e3b8bba7bf95-O52235> with cpus=0 mem=13711, port=1025 from 10.200.0.155 
INFO[0000] Declining offer <a10e0385-8988-4e02-a9c7-e3b8bba7bf95-O52235>
 
INFO[0000] Received Offer <a10e0385-8988-4e02-a9c7-e3b8bba7bf95-O52236> with cpus=2 mem=14863, port=1025 from 10.200.0.154 
INFO[0000] Prepared task: go-task-a0d98708-4b54-4b9c-a1e8-b35c27987b90 with offer a10e0385-8988-4e02-a9c7-e3b8bba7bf95-O52236 for launch
 
INFO[0000] Launching task for offer a10e0385-8988-4e02-a9c7-e3b8bba7bf95-O52236 
INFO[0000] Launch task status: DRIVER_RUNNING           
INFO[0006] Status update: task f28cda6d-1701-4d4c-9def-5068146e7b37  is in state  TASK_RUNNING 
INFO[0006] Server is running
```

As you can see, the Scheduler is registered in the cluster and the first offer is declined as it's not offering any CPU. The second offer is accepted as it fits all our needs: we have set 1 CPU, 512 mb of RAM and one port as needs and the offer has 2 CPU's, 14863 MB or Mem and the port 1025 from 10.200.0.155 machine. We accept this offer and allocate only our needs so that the rest of the offer, that is over our needs, is available for other schedulers or tasks.

Our task is running somewhere. Doing the things correctly we would add this new task to the service discovery and blah blah but for simplicity we have just print the contents of the offer so we can see that it's launched in `10.200.0.157:1025` that were the contents of the offer. If we checked this web server...

```bash
$ curl 10.200.0.157:1025
Hello from Server
```

Done!