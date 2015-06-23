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
func (manager *Manager) requestresources(serv *server.Server) error {
	serverconn, err := net.Dial("tcp", serv.Address)
	if err != nil {
		log.Println("Connection error", err)
		return err
	}
	encoder := gob.NewEncoder(serverconn)
	packet := &server.Packet{server.ServerUsage, nil, nil}
	encoder.Encode(packet)

	dec := gob.NewDecoder(serverconn)
	p := &server.Packet{}
	dec.Decode(p)

	serversnapshot := p.Server
	serv = serversnapshot
	return nil
}

// Gathers resource usage information from all servers.
func (manager *Manager) checkcenterstatus() {
	for {
		manager.Lock()
		for _, server := range manager.Servers {
			go manager.requestresources(server)
		}
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
	log.Println("Manager has received:", p)

	// container request from task
	switch p.Msgtype {
	case server.Newserver: // new server in the network
		manager.Lock()
		serv := p.Server
		//log.Println("Server: ")
		//log.Println(serv)
		manager.Servers = append(manager.Servers, serv)
		log.Printf("Server %s added.", p.Server.Address)
		//log.Println("len(manager.Servers):", len(manager.Servers))
		//log.Println("manager.Servers:", manager.Servers)
		manager.Unlock()
	case server.ContainerRequest: // add or remove container
		manager.Lock()
		if len(manager.Servers) > 0 {
			var serveraddress string
			rand.Seed(time.Now().UnixNano()) // different seed for every iteration

			// uniformfly distributed container allocations
			serv := manager.Servers[rand.Intn(len(manager.Servers))]
			container := p.Container

			// send container allocation
			if exitcode := manager.increaseload(serv, container); exitcode != 0 {
				log.Println("Server ", serveraddress, " is unavailable for requests.")
			}
			log.Printf("Container %d requested.", p.Container.Id)
		}
		manager.Unlock()
	case server.ServerList:
		manager.Lock()
		addresses := []string{}

		for _, server := range manager.Servers {
			addresses = append(addresses, server.Address)
		}
		log.Println("addresses (m):", addresses)
		encoder := gob.NewEncoder(conn)
		encoder.Encode(addresses)
		manager.Unlock()
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
