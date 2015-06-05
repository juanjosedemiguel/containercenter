package main

import (
	"encoding/gob"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"
)

type Taskadministrator struct {
	cpulevels, ramlevels []int
	serverlist           []Serversupervisor // stores server information
}

// Constructs a new SS.
func NewCentermanager(cores, ram, cpulevel, ramlevel int, servertype MsgType) *Taskadministrator {
	ta := Serversupervisor{
		cpulevels:  []int{4},
		ramlevels:  []int{4},
		serverlist: []Serversupervisor{},
	}

	go ta.Run()

	return &ta
}

// Manages load according to the type of processing.
// For simulation, it requests containers from the CM using a Poisson distribution.
func (ta *Taskadministrator) Run() {

}

// Stars TA to
func main() {
	log.Println("Starting Task Administrator (TA)")
	exitcode := 0

	for i := 0; i < 30; i++ {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		cores := rand.Intn(8) + 1        // number of cores allowed to the container [1-8]
		ram := rand.Intn(30) + 2         // ram [2-32 GB]
		requestjson := fmt.Sprint(`{"ip":"none","cores":`, strconv.Itoa(cores), `,"ram":`, strconv.Itoa(ram), `}`)

		log.Println(cores)
		log.Println(ram)
		log.Println(requestjson)
		if exitcode = message.Send(message.Packet{1, requestjson}, "localhost", 8080); exitcode > 0 {
			// message transmission failed
			log.Println("Container request (" + i + ") failed.")
		}
		log.Println("i: ", i)
		time.Sleep(1000 * time.Millisecond)
	}

	log.Println("Done.")
}
