package server

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
)

type Container struct {
	Id                 int
	Cores, Memory      int
	Cpulevel, Ramlevel int
	Timetolive         int
}

type Server struct {
	Id, Cores, Memory, Cpulevel, Ramlevel int
	Cputhreshold, Ramthreshold            int
	Servertype                            message.ServerType
	Address, Manageraddress               string
	Containers                            []*Container
}

// Constructs a new SS.
func NewServer(id, cores, memory int, address, manageraddress string, servertype message.ServerType) *Server {
	server := Server{
		Id:             id,
		Cores:          cores,
		Memory:         memory,
		Servertype:     servertype,
		Address:        address,
		Manageraddress: manageraddress,
		Containers:     []*Container{},
	}

	switch servertype {
	case 0: // compute intensive
		server.Cputhreshold = 50
		server.Ramthreshold = 10
	case 1: // Memory intensive
		server.Cputhreshold = 10
		server.Ramthreshold = 50
	case 2: // combined
		server.Cputhreshold = 30
		server.Ramthreshold = 30
	}
	return &server
}

// Adds a container with a configuration specified by the TA.
func (server Server) addcontainer(container Container) {
	server.Containers = append(server.Containers, &container)
}

// Finds the container with the specified ID.
func (server *Server) getcontainer(containerid int) (container *Container) {
	for _, container := range server.Containers {
		if containerid == container.Id {
			return container
		}
	}
	return nil
}

// Selects best container according to alert.
func (server *Server) selectcontainer(alert int) (candidateid int) {
	candidatescore := 0

	switch alert {
	case 1: // cpu
		for _, elem := range server.Containers {
			if candidatescore < elem.Cores {
				candidatescore = elem.Cores
				candidateid = elem.Id
			}
		}
	case 2: // ram
		for _, elem := range server.Containers {
			if candidatescore < elem.Memory {
				candidatescore = elem.Memory
				candidateid = elem.Id
			}
		}
	case 3: // combined
		for _, elem := range server.Containers {
			if aux := elem.Cores * elem.Memory; candidatescore < aux {
				candidatescore = elem.Cores
				candidateid = elem.Id
			}
		}
	}
	return
}

// Triggers migration threshold alert.
func (server *Server) triggeralert() {
	var alert int

	for {
		if server.Cpulevel > server.Cputhreshold {
			if server.Ramlevel > server.Ramthreshold {
				alert = 3 // both
			} else {
				alert = 1 // cpu only
			}
		} else if server.Ramlevel > server.Ramthreshold {
			alert = 2 // ram only
		} else {
			alert = 0 // normal status
		}

		if alert > 0 {
			err := server.migratecontainer(alert) // blocks method until container is migrated ignoring alarms in the meantime
			if err != nil {
				fmt.Println(err)
			} else {
				alert = 0
			}
		}

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Starts migration of container using the WBP protocol.
func (server *Server) migratecontainer(alert int) (err error) {
	// select best container for migration according to alert
	containerid := server.selectcontainer(alert)
	container := server.getcontainer(containerid)

	// get servers in network
	conn, err := net.Dial("tcp", server.Manageraddress)
	if err != nil {
		fmt.Println("Connection error", err)
		return err
	}
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	p := &message.Packet{message.ServerList, nil}
	encoder.Encode(p)

	dec := gob.NewDecoder(conn)
	addresses := []string{}
	dec.Decode(addresses)

	// call for proposals
	candidates := make([]Server, 0)
	for _, serveraddress := range addresses {
		serverconn, err := net.Dial("tcp", serveraddress)
		if err != nil {
			fmt.Println("Connection error", err)
			return err
		}

		encoder := gob.NewEncoder(serverconn)
		packet := &message.Packet{message.CallForProposals, container}
		encoder.Encode(packet)

		dec := gob.NewDecoder(serverconn)
		packet = &message.Packet{}
		dec.Decode(packet)
		if p.Msgtype == message.Proposal {
			candidates = append(candidates, packet.Data.(Server)) // snapshot of candidate server
		}
		serverconn.Close()
	}

	// pick best candidate
	migrationdone := false
	for migrationdone == false || len(candidates) == 0 {
		Address := ""
		i := 0
		switch alert {
		case 1: // pick the server with the lowest CPU usage
			Cpulevel := 100
			for c, candidate := range candidates {
				if candidate.Cpulevel < Cpulevel {
					Address = candidate.Address
					i = c
				}
			}
		case 2: // pick the server with the lowest RAM usage
			Ramlevel := 100
			for c, candidate := range candidates {
				if candidate.Ramlevel < Ramlevel {
					Address = candidate.Address
					i = c
				}
			}
		case 3: // pick the server with the lowest combined usage
			combinedlevel := 10000
			for c, candidate := range candidates {
				if ((candidate.Cpulevel + 1) * (candidate.Ramlevel + 1)) < combinedlevel {
					Address = candidate.Address
					i = c
				}
			}
		}

		// confirm best candidate can still host the container
		// delete candidate from list
		candidates[i], candidates = candidates[len(candidates)-1], candidates[:len(candidates)-1]

		candidateconn, err := net.Dial("tcp", Address)
		if err != nil {
			fmt.Println("Connection error", err)
			return err
		}
		defer candidateconn.Close()

		encoder := gob.NewEncoder(candidateconn)
		packet := &message.Packet{message.Accepted, container}
		encoder.Encode(packet)

		dec = gob.NewDecoder(candidateconn)
		p = &message.Packet{}
		dec.Decode(p)

		// candidate is able to host
		if p.Msgtype == message.Accepted {
			// migrate container
			p = &message.Packet{message.Migration, container}
			encoder.Encode(p)
			// inform migration completed
			p = &message.Packet{message.MigrationDone, "Done!"}
			encoder.Encode(p)
			container.Timetolive = 0
			migrationdone = true
		} else {
			log.Println("Migration cancelled.")
		}
	}
	if !migrationdone {
		err = fmt.Errorf("Migration failed.")
	}
	return
}

// Updates the time to live of every container.
func (server *Server) updatecontainers() {

	for i, container := range server.Containers {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		container.Cpulevel += rand.Intn(7) - 2
		rand.Seed(time.Now().UnixNano()) // independent variables
		container.Ramlevel += rand.Intn(5) - 1

		if container.Cpulevel < 0 {
			container.Cpulevel = 0
		} else if container.Cpulevel > 100 {
			log.Printf("Container %d from server %d exceeded CPU limit (100%).", i)
			container.Timetolive = 0
		}

		if container.Ramlevel < 0 {
			container.Ramlevel = 0
		} else if container.Ramlevel > 100 {
			log.Printf("Container %d from server %d exceeded RAM limit (100%).", i)
			container.Timetolive = 0
		}

		container.Timetolive--
	}
}

// Updates each container's time to live.
func (server *Server) checkexpiredcontainers() {
	for {
		server.updatecontainers()
		alivecontainers := make([]*Container, len(server.Containers))
		for _, container := range server.Containers {
			if container.Timetolive > 0 {
				alivecontainers = append(alivecontainers, container)
			}
		}
		server.Containers = alivecontainers

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Checks if the server can host a proposed container.
func (server *Server) canhost(container Container) (canhost bool) {
	canhost = false
	coresused := 0
	memoryused := 0
	for _, container := range server.Containers {
		coresused += container.Cores
		memoryused += container.Memory
	}
	if (coresused+container.Cores) <= server.Cores && (memoryused+container.Memory) <= server.Memory {
		canhost = true
	}
	return
}

// Updates server resource usage and sends snapshot to manager.
func (server *Server) updatesupervisorstatus() {
	cpu, ram := 0, 0
	for _, container := range server.Containers {
		cpu += container.Cpulevel
		ram += container.Ramlevel
	}
	server.Cpulevel = int((float64(cpu) / float64(len(server.Containers))) * 100)
	server.Ramlevel = int((float64(ram) / float64(len(server.Containers))) * 100)
}

// Handles inputs and routes them to the corresponding functions of the server.
func (server *Server) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)
	encoder := gob.NewEncoder(conn)
	defer conn.Close()

	remoteaddress := conn.RemoteAddr().String()
	switch p.Msgtype {
	case message.ContainerAllocation: // container allocated by manager
		server.addcontainer(p.Data.(Container))
	case message.ServerUsage: // resource usage request from manager

		p = &message.Packet{message.ServerUsage, server}
		encoder.Encode(p)
	case message.CallForProposals:
		if server.canhost(p.Data.(Container)) {
			// send snapshot
			p = &message.Packet{message.Proposal, server}
			encoder.Encode(p)

		} else {
			// send refusal
			p = &message.Packet{message.Rejected, nil}
			encoder.Encode(p)
		}

	case message.Accepted:
		if server.canhost(p.Data.(Container)) {
			// send confirmation
			p = &message.Packet{message.Accepted, nil}
			encoder.Encode(p)

			// wait for container and add it to server
			p = &message.Packet{}
			dec.Decode(p)
			server.addcontainer(p.Data.(Container))

			// wait for migration confirmation
			p = &message.Packet{}
			dec.Decode(p)

			if p.Msgtype == message.MigrationDone {
				log.Printf("Container %d migrated from %s to %s.", p.Data.(Container).Id, remoteaddress, server.Address)
			}
		} else {
			//  send cancelation
			p = &message.Packet{message.Rejected, nil}
			encoder.Encode(p)
		}

	}
}

// Sets up handling for incoming connections and run monitoring goroutines.
func (server *Server) Run() {
	log.Printf("Starting Server %d", server.Id)

	// monitoring methods
	go server.updatesupervisorstatus()
	go server.checkexpiredcontainers()
	go server.triggeralert()
	log.Println("Serveraddress:", server.Address)
	ln, err := net.Listen("tcp", server.Address)
	if err != nil {
		log.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			log.Println("Connection error", err)
			continue
		}
		go server.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}
}
