package main

import (
	"github.com/mesos/mesos-go/mesosproto"
	"github.com/golang/protobuf/proto"
	"github.com/mesos/mesos-go/mesosutil"
	"github.com/mesos/mesos-go/scheduler"
	"flag"
	"net"

	log "github.com/Sirupsen/logrus"
	"os"
)

var (
	master       = flag.String("master", "127.0.0.1:5050", "Master address <ip:port>")
)

func init() {
	flag.Parse()
}

func main() {
	//ExecutorInfo
	executorUri := "https://s3-eu-west-1.amazonaws.com/enablers/executor"
	executorUris := []*mesosproto.CommandInfo_URI{
		{
			Value:      &executorUri,
			Executable: proto.Bool(true),
		},
	}

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
	my_scheduler := NewExampleScheduler(executorInfo, 1, 128)


	//Framework
	frameworkInfo := &mesosproto.FrameworkInfo{
		User: proto.String(""), // Mesos-go will fill in user.
		Name: proto.String("Stratio Server Framework (Go)"),
	}

	//Scheduler Driver
	config := scheduler.DriverConfig{
		Scheduler:      my_scheduler,
		Framework:      frameworkInfo,
		Master:         *master,
		Credential:     (*mesosproto.Credential)(nil),
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
}

func parseIP(address string) net.IP {
	addr, err := net.LookupIP(address)
	if err != nil {
		log.Fatal(err)
	}
	if len(addr) < 1 {
		log.Fatalf("failed to parse IP from address '%v'", address)
	}
	return addr[0]
}
