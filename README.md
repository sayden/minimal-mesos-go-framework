# minimal-mesos-go-framework

Minimally possible mesos framework


## How to create a Mesos framework in Go

The process involved in create a Mesos framework is divided in two steps: create a Mesos Scheduler and a Mesos Executor.


## Creating a Mesos Scheduler
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
```

Here, we'll set the URL where the cluster can download the executor in the variable "executorUri" and fill a CommandInfo_URI struct. We also set that the downloaded file is an executable (you could also set it as a compressed file so that it extracts it into the sandbox). Once we have the URI's, we created an ExecutorInfo to set the information we mention above.

### Filling Framework information
```go

//Framework
frameworkInfo := &mesosproto.FrameworkInfo{
   User: proto.String("root"), // Mesos-go will fill in user.
   Name: proto.String("Stratio Server Framework (Go)"),
}
```

### Creating an Example Scheduler
Now we'll create a struct that will implement "SchedulerDriver" interface:

```go
type ExampleScheduler struct{
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
       offeredPort[0].GetBegin())


      //Decline offer if the offer doesn't satisfy our needs
      if offeredCpu < s.NeededCpu || offeredMem < s.NeededRam {
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

As you can see, we have implemented extensively one method (ResourceOffers) and the rest simply do some logging.
* _StatusUpdate_: This method will be called when the Executor notify scheduler with some update on their status. We'll see later that this status update and information must be sent manually when implementing the Executor. In our example we don't do anything special, We simply setup some logging depending on the status received. Pay attention that we abort the driver when a "Lost", "Killed" or "Failed" status is received.
* _ResourceOffers_: ResourceOffers will be called on every offer from the cluster, even when executor are already running and is up to you to refuse offers once you have launched all executors you need. Let's analyze it step by step:

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

Next, we checked the contents of the offer. In the offer, each resource is of one type (cpu, mem, port) defined by the field "Name" and recovered using "GetName()"

```go
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
       offeredPort[0].GetBegin())
...
...
.......
```
