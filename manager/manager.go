package manager

import (
	"encoding/gob"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"github.com/juanjosedemiguel/loadbalancingsim/server"
)

type Manager struct {
	servers []*server.Server
}

// Constructs a new manager.
func NewManager() *Manager {
	manager := Manager{
		servers: []*server.Server{},
	}

	return &manager
}

// Allocates a new container to increase the server's workload.
func (manager *Manager) increaseload(server *server.Server, container server.Container) int {
	return message.Send(message.Packet{message.ContainerAllocation, container}, server.Address)
}

// Requests resource usage information from a server.
func (manager *Manager) requestresources(server *server.Server) {
	exitcode := message.Send(message.Packet{message.ServerUsage, nil}, server.Address)
	if exitcode != 0 {
		log.Println("Server ", server.Address, " is unavailable for requests.")
	}
}

// Gathers resource usage information from all servers.
func (manager *Manager) checkcenterstatus() {
	for _, server := range manager.servers {
		go manager.requestresources(server)
	}
	time.Sleep(time.Duration(1000) * time.Millisecond) // wait for next resource usage update
}

// Handles inputs from the task and routes them to the corresponding functions of the manager.
func (manager *Manager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)
	defer conn.Close()

	// container request from task
	switch p.Msgtype {
	case message.ContainerRequest: // add or remove container
		var serveraddress string
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration

		// uniformfly distributed container allocations
		serv := manager.servers[rand.Intn(len(manager.servers))]
		container := p.Data.(server.Container)

		// send container allocation
		if exitcode := manager.increaseload(serv, container); exitcode != 0 {
			log.Println("Server ", serveraddress, " is unavailable for requests.")
		}

	case message.ServerUsage: // server usage information received.
		server := p.Data.(server.Server)
		manager.servers = append(manager.servers, &server)
	case message.ServerList:
		addresses := make([]string, len(manager.servers))

		for _, server := range manager.servers {
			addresses = append(addresses, server.Address)
		}
		encoder := gob.NewEncoder(conn)
		encoder.Encode(addresses)
	}
}

// Sets up handling for incoming connections.
func (manager *Manager) Run(port int) {
	go manager.checkcenterstatus()

	log.Println("Starting Center Manager (CM)")
	ln, err := net.Listen("tcp", ":8080")
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
}
