package task

import (
	"log"
	"math/rand"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"github.com/juanjosedemiguel/loadbalancingsim/server"
)

type Task struct {
	name     string
	requests int
}

// Constructs a new Task.
func NewTask(name string, requests int) *Task {
	t := Task{
		name:     name,
		requests: requests,
	}
	return &t
}

// Manages load according to the type of processing.
// For this simulation, it requests containers from manager.go every 100 ms.
// Each container executes for 50 s.
func (t *Task) Run() {
	time.Sleep(6 * time.Second)
	log.Println("Starting Task")
	exitcode := 0

	// requests containers with random configurations
	for i := 0; i < t.requests; i++ {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		cores := rand.Intn(8) + 1        // cores allowed for the container [1-8]
		memory := rand.Intn(30) + 2      // memory allowed for the container [2-32 GB]
		container := server.Container{
			Id:         i,
			Cores:      cores,
			Memory:     memory,
			Cpulevel:   3,
			Ramlevel:   3,
			Timetolive: 50,
		}

		// container request failed
		if exitcode = message.Send(message.Packet{message.ContainerRequest, container}, "localhost:8080"); exitcode > 0 {
			log.Printf("Container request (%d) failed.", i)
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("Task is done requesting.")
}
