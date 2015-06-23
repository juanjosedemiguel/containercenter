package server

import (
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Packet struct {
	Msgtype   MsgType
	Container *Container
	Server    *Server
}

type MsgType uint8

const (
	Newserver MsgType = iota
	ContainerRequest
	ContainerAllocation
	ServerUsage
	ServerList
	CallForProposals
	Proposal
	Accepted
	Rejected
	Migration
	MigrationDone
)

type ServerType uint8

const (
	HighCPU    ServerType = iota // = 0 (compute intensive)
	HighMemory                   // = 1 (memory intensive)
	Combined                     // = 2 (combined)
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
	Servertype                            ServerType
	Address, Manageraddress               string
	Containers                            []*Container
	mutex                                 sync.Mutex
}

// Sends a message that consists of a Packet struct to a specified server.
func Send(packet Packet, serveraddress string) (exitcode int) {
	exitcode = 0
	conn, err := net.Dial("tcp", serveraddress)
	if err != nil {
		fmt.Println("Connection error", err)
		exitcode = 1
	}
	encoder := gob.NewEncoder(conn)
	p := &packet
	encoder.Encode(p)
	defer conn.Close()
	return
}

// Constructs a new SS.
func NewServer(id, cores, memory int, address, manageraddress string, servertype ServerType) *Server {
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
func (server *Server) addcontainer(container *Container) {
	server.mutex.Lock()
	server.Containers = append(server.Containers, container)
	server.mutex.Unlock()
}

// Finds the container with the specified ID.
func (server *Server) getcontainer(containerid int) (container *Container) {
	server.mutex.Lock()
	for _, container := range server.Containers {
		if containerid == container.Id {
			return container
		}
	}
	server.mutex.Unlock()
	return nil
}

// Selects best container according to alert.
func (server *Server) selectcontainer(alert int) (candidateid int) {
	server.mutex.Lock()
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
			if aux := (elem.Cores + 1) * (elem.Memory + 1); candidatescore < aux {
				candidatescore = elem.Cores
				candidateid = elem.Id
			}
		}
	}
	server.mutex.Unlock()
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
				log.Println("Migration succesful.")
			}
		}

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Starts migration of container using the WBP protocol.
func (server *Server) migratecontainer(alert int) (err error) {
	log.Println("Migration started.", alert)
	// select best container for migration according to alert
	containerid := server.selectcontainer(alert)
	container := server.getcontainer(containerid)

	// get servers in network
	log.Println("Getting available servers.", alert)
	conn, err := net.Dial("tcp", server.Manageraddress)
	if err != nil {
		fmt.Println("Connection error", err)
		return err
	}
	encoder := gob.NewEncoder(conn)
	p := &Packet{ServerList, nil, nil}
	err = encoder.Encode(p)
	if err != nil {
		log.Println(err)
	}
	dec := gob.NewDecoder(conn)
	addresses := []string{}
	err = dec.Decode(&addresses)
	if err != nil {
		log.Println("addreses (s)", err)
	}

	log.Println("addresses:", addresses)
	conn.Close()

	// call for proposals
	log.Println("Getting candidates.", alert)
	candidates := make([]*Server, 0)
	for _, serveraddress := range addresses {
		if serveraddress == server.Address {
			continue
		}

		serverconn, err := net.Dial("tcp", serveraddress)
		if err != nil {
			fmt.Println("Connection error", err)
			return err
		}

		encoder := gob.NewEncoder(serverconn)
		packet := &Packet{CallForProposals, container, nil}
		err = encoder.Encode(packet)
		if err != nil {
			return err
		}
		log.Println("serverconn", serverconn)

		dec := gob.NewDecoder(serverconn)
		packet = &Packet{}
		err = dec.Decode(packet)
		if err != nil {
			return err
		}
		log.Println("ano", packet)
		if packet.Msgtype == Proposal {
			candidates = append(candidates, packet.Server) // snapshot of candidate server
			log.Println("Candidate added.", alert)
		}
		serverconn.Close()
	}

	log.Println("candidates:", candidates)

	// pick best candidate
	migrationdone := false
	for migrationdone == false && len(candidates) != 0 {
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
				log.Println("candidate levels:", candidate.Cpulevel, candidate.Ramlevel)
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
			fmt.Println("Connection error", err, i)
			return err
		}
		defer candidateconn.Close()

		encoder := gob.NewEncoder(candidateconn)
		packet := &Packet{Accepted, container, nil}
		err = encoder.Encode(packet)
		if err != nil {
			return err
		}

		dec = gob.NewDecoder(candidateconn)
		p = &Packet{}
		err = dec.Decode(p)
		if err != nil {
			return err
		}

		// candidate is able to host
		if p.Msgtype == Accepted {
			// migrate container
			p = &Packet{Migration, container, nil}
			err = encoder.Encode(p)
			if err != nil {
				return err
			}
			// inform migration completed
			p = &Packet{MigrationDone, nil, nil}
			err = encoder.Encode(p)
			if err != nil {
				return err
			}
			container.Timetolive = 0
			migrationdone = true
		} else {
			log.Println("Migration canceled.")
		}
	}
	if !migrationdone {
		err = fmt.Errorf("Migration failed: %v %d.", migrationdone, len(candidates))
	}
	return
}

// Updates the time to live of every container.
func (server *Server) updatecontainers() {
	server.mutex.Lock()
	log.Println("( o ) o )", server.Containers)
	for i, container := range server.Containers {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		container.Cpulevel += rand.Intn(7) - 2
		rand.Seed(time.Now().UnixNano()) // independent variables
		container.Ramlevel += rand.Intn(5) - 1

		log.Println("-> ", container.Id, container.Cpulevel, container.Ramlevel)

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
	server.mutex.Unlock()
}

// Updates each container's time to live.
func (server *Server) checkexpiredcontainers() {
	for {
		server.updatecontainers()
		server.mutex.Lock()
		alivecontainers := make([]*Container, 0)
		for _, container := range server.Containers {
			if container.Timetolive > 0 {
				alivecontainers = append(alivecontainers, container)
			}
		}
		server.Containers = alivecontainers
		server.mutex.Unlock()
		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Checks if the server can host a proposed container.
func (server *Server) canhost(container *Container) (canhost bool) {
	canhost = false
	coresused := 0
	memoryused := 0
	for _, container := range server.Containers {
		coresused += container.Cores
		memoryused += container.Memory
	}
	log.Println("(coresused+container.Cores):", (coresused + container.Cores), "(server.Cores):", server.Cores, "(memoryused+container.Memory):", (memoryused + container.Memory), "server.Memory:", server.Memory)
	if (coresused+container.Cores) <= server.Cores && (memoryused+container.Memory) <= server.Memory {
		canhost = true
	}
	return
}

// Updates server resource usage.
func (server *Server) readserverresources() {
	for {
		server.mutex.Lock()
		cpu, ram := 0, 0
		for _, container := range server.Containers {
			cpu += container.Cpulevel
			ram += container.Ramlevel
			log.Println("	->", container.Id, cpu, ram)
		}
		n := len(server.Containers)
		if n == 0 {
			server.Cpulevel = 0
			server.Ramlevel = 0
		} else {
			server.Cpulevel = int((float64(cpu) / float64(n)))
			server.Ramlevel = int((float64(ram) / float64(n)))
		}
		log.Println("	->", server.Cpulevel, server.Ramlevel)
		server.mutex.Unlock()

		time.Sleep(1000 * time.Millisecond)
	}
}

// Handles inputs and routes them to the corresponding functions of the server.
func (server *Server) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &Packet{}
	dec.Decode(p)
	encoder := gob.NewEncoder(conn)
	defer conn.Close()

	log.Println("Server has received:", p)
	var err error

	remoteaddress := conn.RemoteAddr().String()
	switch p.Msgtype {
	case ContainerAllocation: // container allocated by manager
		log.Println("Container allocated:", p.Container.Id, p.Container.Cores, p.Container.Memory, p.Container.Cpulevel, p.Container.Ramlevel, p.Container.Timetolive)
		server.addcontainer(p.Container)
	case ServerUsage: // resource usage request from manager
		//log.Println("Serversnap: ")
		//log.Println(server)
		p = &Packet{ServerUsage, nil, server}
		encoder.Encode(p)
	case CallForProposals:
		log.Println("Server has received callforproposals.")
		if server.canhost(p.Container) {
			log.Println("canhost!!!!!!!")
			// send snapshot
			p = &Packet{Proposal, nil, server}
			err = encoder.Encode(p)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("canhost", p)
			}
		} else {
			// send refusal
			p = &Packet{Rejected, nil, nil}
			err = encoder.Encode(p)
			if err != nil {
				log.Println(err)
			}
		}

	case Accepted:
		log.Println("Server has received accepted.")
		if server.canhost(p.Container) {
			// send confirmation
			p = &Packet{Accepted, nil, nil}
			err = encoder.Encode(p)
			if err != nil {
				log.Println(err)
			}

			// wait for container and add it to server
			p = &Packet{}
			log.Println("container added 0")
			err = dec.Decode(p)
			if err != nil {
				log.Println(err)
			}
			log.Println("container added 1")
			server.addcontainer(p.Container)
			log.Println("container added 2")

			// wait for migration confirmation
			conf := &Packet{}
			err = dec.Decode(conf)
			if err != nil {
				log.Println(err)
			}
			log.Println("p", p)
			log.Println("conf", conf)
			if conf.Msgtype == MigrationDone {
				log.Printf("Container %d migrated from %s to %s.", p.Container.Id, remoteaddress, server.Address)
			}
		} else {
			//  send cancelation
			p = &Packet{Rejected, nil, nil}
			err = encoder.Encode(p)
			if err != nil {
				log.Println(err)
			}
		}

	}
}

// Sets up handling for incoming connections and run monitoring goroutines.
func (server *Server) Run() {
	log.Printf("Starting Server %d...", server.Id)
	log.Println(server)

	// registers server in the manager's list
	Send(Packet{Newserver, nil, server}, server.Manageraddress)

	// monitoring methods
	go server.readserverresources()
	go server.checkexpiredcontainers()
	go server.triggeralert()

	log.Printf("Server %d started.", server.Id)
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
