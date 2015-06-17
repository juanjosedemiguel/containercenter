package main

import (
	"encoding/gob"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"github.com/juanjosedemiguel/loadbalancingsim/server"
	"github.com/juanjosedemiguel/loadbalancingsim/task"
)

type Manager struct {
	servers         []*server.Supervisor // Supervisor instances involved in the simulation
	serversnapshots []map[string]interface{}
}

// Constructs a new Center Manager.
func NewManager() *Manager {
	manager := Manager{
		servers:         []server.Supervisor{},
		serversnapshots: []map[string]interface{}{},
	}

	return &manager
}

// Allocates a new container to a server (SS) to increase it's workload.
func (manager *Manager) increaseload(server map[string]interface{}, containerconfig string) int {
	return message.Send(message.Packet{2, containerconfig}, server["address"].(string), server["port"].(int))
}

// Requests resource usage information from a Server Supervisor (SS).
func (manager *Manager) requestresources(server map[string]interface{}) {
	exitcode := message.Send(message.Packet{3, ""}, server["address"].(string), server["port"].(int))
	if exitcode != 0 {
		log.Println("Server ", server["address"].(string), " is unavailable for requests.")
	}
}

// Gathers resource usage information from all servers (SS).
func (manager *Manager) checkcenterstatus() {
	for _, server := range manager.serversnapshots {
		go manager.requestresources(server)
	}
	time.Sleep(time.Duration(1000) * time.Millisecond) // wait for next resource usage update
}

// Handles inputs from the Task Administrator (TA) and routes them to the corresponding functions of the Center Manager (CM).
func (manager *Manager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)
	// decoding JSON string
	datajson := message.Decodepacket(*p)

	var exitcode int
	// container request from TA
	switch p.Msgtype {
	case ContainerRequest: // add or remove container
		var serveraddress string
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration

		// uniformfly distributed container allocations
		server := manager.serversnapshots[rand.Intn(len(manager.serversnapshots))]
		serveraddress = server["address"].(string)
		containerconfig := p.Data
		exitcode = manager.increaseload(server, containerconfig)

		// send container allocation
		if exitcode = manager.increaseload(server, containerconfig); exitcode != 0 {
			log.Println("Server ", serveraddress, " is unavailable for requests.")
		}

	case ServerUsage: // server usage information received.
		serverid := datajson["id"].(int)
		manager.serversnapshots[serverid] = datajson
	}
}

// Sets up handling for incoming connections.
func (manager *Manager) Run(port int) {
	log.Println("Starting Center Manager (CM)")
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			log.Println("Connection error", err)
			continue
		}
		go manager.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}

	go manager.checkcenterstatus()
}

// Sets up system - loads Server Supervisors (SS).
func main() {
	// Starts the Center Manager
	manager := NewManager()
	go manager.Run(8080)

	// 32 cores, 128 GB of RAM (one of each server type)
	types = []message.ServerType{message.HighCPU, message.HighMemory, message.Combined}
	for i, servertype := range types {
		manager.servers = append(manager.servers, server.NewSupervisor(i, 32, 128, "localhost", 8081+i, servertype))
		go server.Run()
	}

	// starts Task Administrator
	admin := task.NewAdministrator()
	admin.Run()

}
