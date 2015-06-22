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
	Servers []*server.Server
}

// Constructs a new manager.
func NewManager() *Manager {
	manager := Manager{
		Servers: []*server.Server{},
	}

	return &manager
}

// Allocates a new container to increase the server's workload.
func (manager *Manager) increaseload(server *server.Server, container *server.Container) int {
	return message.Send(message.Packet{message.ContainerAllocation, container, nil}, server.Address)
}

// Requests resource usage information from a server.
func (manager *Manager) requestresources(serv *server.Server) error {
	serverconn, err := net.Dial("tcp", serv.Address)
	if err != nil {
		log.Println("Connection error", err)
		return err
	}
	encoder := gob.NewEncoder(serverconn)
	packet := &message.Packet{message.ServerUsage, nil, nil}
	encoder.Encode(packet)

	dec := gob.NewDecoder(serverconn)
	p := &message.Packet{}
	dec.Decode(p)

	serversnapshot := p.Server
	manager.Servers = append(manager.Servers, &serversnapshot)
	return nil
}

// Gathers resource usage information from all servers.
func (manager *Manager) checkcenterstatus() {
	for {
		for _, server := range manager.Servers {
			go manager.requestresources(server)
		}
		time.Sleep(time.Duration(1000) * time.Millisecond) // wait for next resource usage update
	}
}

// Handles inputs from the task and routes them to the corresponding functions of the manager.
func (manager *Manager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)
	defer conn.Close()
	log.Println("Manager has received:", p)
	// container request from task
	switch p.Msgtype {
	case message.NewServer: // new server in the network
		serversnapshot := p.Server
		manager.Servers = append(manager.Servers, &serversnapshot)
		log.Println("Server added.")
	case message.ContainerRequest: // add or remove container
		log.Println("len(manager.Servers):", len(manager.Servers))
		if len(manager.Servers) > 0 {
			var serveraddress string
			rand.Seed(time.Now().UnixNano()) // different seed for every iteration

			// uniformfly distributed container allocations
			log.Println("len(manager.Servers):", len(manager.Servers))
			serv := manager.Servers[rand.Intn(len(manager.Servers))]
			container := p.Container

			// send container allocation
			if exitcode := manager.increaseload(serv, container); exitcode != 0 {
				log.Println("Server ", serveraddress, " is unavailable for requests.")
			}
		}
	case message.ServerList:
		addresses := make([]string, len(manager.Servers))

		for _, server := range manager.Servers {
			addresses = append(addresses, server.Address)
		}
		encoder := gob.NewEncoder(conn)
		encoder.Encode(addresses)
	}
}

// Sets up handling for incoming connections.
func (manager *Manager) Run() {
	log.Println("Starting Manager")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Println("Connection error", err)
	}

	go manager.checkcenterstatus()

	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			log.Println("Connection error", err)
			continue
		}
		go manager.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}
}
