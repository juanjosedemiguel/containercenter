package task

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
)

type Administrator struct {
	name string
}

// Constructs a new SS.
func NewAdministrator(name string) *Administrator {
	a := Administrator{
		name: name,
	}
	return &a
}

// Manages load according to the type of processing.
// For this simulation, it requests containers from the CM every 100 ms.
// Each container executes for 50 s.
func (a *Administrator) Run() {
	log.Println("Starting Task Administrator (TA)")
	exitcode := 0

	// requests 30 containers with random configurations
	for i := 0; i < 3; i++ {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		cores := rand.Intn(8) + 1        // cores allowed for the container [1-8]
		memory := rand.Intn(30) + 2      // memory allowed for the container [2-32 GB]
		requestjson := fmt.Sprintf(`{"id":%d,"cores":%d,"memory":%d,"cpulevel":%d,"ramlevel":%d,"timetolive":%d}`, i, cores, memory, 3, 3, 50)

		// container request failed
		if exitcode = message.Send(message.Packet{message.ContainerRequest, requestjson}, "localhost", 8080); exitcode > 0 {
			log.Println("Container request (" + i + ") failed.")
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("TA is done requesting.")
}

// Stars TA and executes method that requests containers.
func main() {
	ta := NewAdministrator("admin")
	ta.Run()
}
