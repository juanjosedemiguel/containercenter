package manager

import (
	"encoding/gob"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/server"
)

var (
	Verbose bool = false
)

type Manager struct {
	Servers []*server.Server
	sync.Mutex
}

// Constructs a new manager.
func NewManager() *Manager {
	manager := Manager{
		Servers: []*server.Server{},
	}

	return &manager
}

// Allocates a new container to increase the server's workload.
func (manager *Manager) increaseload(serv *server.Server, container *server.Container) int {
	return server.Send(server.Packet{server.ContainerAllocation, container, nil}, serv.Address)
}

// Requests resource usage information from a server.
func (manager *Manager) requestresources(serv *server.Server) *server.Server {
	serverconn, err := net.Dial("tcp", serv.Address)
	if err != nil {
		log.Println("Connection error", err)
	}
	encoder := gob.NewEncoder(serverconn)
	packet := &server.Packet{server.ServerUsage, nil, nil}
	encoder.Encode(packet)

	dec := gob.NewDecoder(serverconn)
	p := &server.Packet{}
	dec.Decode(p)
	serversnapshot := p.Server
	return serversnapshot
}

// Gathers resource usage information from all servers.
func (manager *Manager) checkcenterstatus() {
	for {
		manager.Lock()
		log.Println("Servers:")
		for i, server := range manager.Servers {
			server = manager.requestresources(server)
			log.Printf("Server %d containers:%+v\n", i, server.Containers)
		}
		log.Println("Done.")
		manager.Unlock()
		time.Sleep(time.Duration(1000) * time.Millisecond) // wait for next resource usage update
	}
}

// Handles inputs from the task and routes them to the corresponding functions of the manager.
func (manager *Manager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &server.Packet{}
	dec.Decode(p)
	defer conn.Close()

	if Verbose {
		log.Println("Manager has received:", p.Msgtype, p.Container, p.Server)
	}

	// container request from task
	switch p.Msgtype {
	case server.Newserver: // new server in the network
		if Verbose {
			log.Println("Manager has received Newserver.")
		}
		manager.Lock()
		serv := p.Server
		manager.Servers = append(manager.Servers, serv)
		if Verbose {
			log.Printf("Server %s added.", p.Server.Address)
		}
		manager.Unlock()
	case server.ContainerRequest: // add or remove container
		if Verbose {
			log.Println("Manager has received Containerrequest.")
		}
		manager.Lock()
		if len(manager.Servers) > 0 {
			var serveraddress string
			rand.Seed(time.Now().UnixNano()) // different seed for every iteration

			// uniformfly distributed container allocations
			serv := manager.Servers[rand.Intn(len(manager.Servers))]
			//serv := manager.Servers[0]
			container := p.Container

			// send container allocation
			if exitcode := manager.increaseload(serv, container); exitcode != 0 {
				log.Printf("Server %s is unavailable for requests.", serveraddress)
			}
			if Verbose {
				log.Printf("Container %d requested.", p.Container.Id)
			}
		}
		manager.Unlock()
	case server.ServerList:
		if Verbose {
			log.Println("Manager has received Serverlist.")
		}
		manager.Lock()
		addresses := []string{}

		for _, server := range manager.Servers {
			addresses = append(addresses, server.Address)
		}

		encoder := gob.NewEncoder(conn)
		encoder.Encode(addresses)
		manager.Unlock()
	}
}

// Sets up handling for incoming connections.
func (manager *Manager) Run() {
	if Verbose {
		log.Println("Starting Manager")
	}
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
