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
