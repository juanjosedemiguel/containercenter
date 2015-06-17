package server

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/juanjosedemiguel/loadbalancingsim/message"
)

type Container struct {
	cores, memory      int
	cpulevel, ramlevel int
	timetolive         int
}

type Supervisor struct {
	id, cores, memory, cpulevel, ramlevel, port int
	servertype                                  message.ServerType
	address                                     string
	cputhreshold, ramthreshold                  int
	intervalstart                               time.Time
	containers                                  []*Container
}

// Constructs a new SS.
func NewSupervisor(id, cores, memory int, address string, port int, servertype message.ServerType) *Supervisor {
	ss := Supervisor{
		id:             id,
		cores:          cores,
		memory:         memory,
		servertype:     servertype,
		address:        address,
		manageraddress: "localhost:8080",
		port:           port,
		containers:     []Container{},
	}

	switch servertype {
	case 0: // compute intensive
		ss.cputhreshold = 50
		ss.ramthreshold = 10
	case 1: // memory intensive
		ss.cputhreshold = 10
		ss.ramthreshold = 50
	case 2: // combined
		ss.cputhreshold = 30
		ss.ramthreshold = 30
	}
	return &ss
}

// Adds a container with a configuration specified by the TA.
func (ss Supervisor) addcontainer(containerconfig map[string]interface{}) {
	ss.containers = append(ss.containers,
		Container{
			id:         containerconfig["id"],
			cores:      containerconfig["cores"],
			memory:     containerconfig["memory"],
			cpulevel:   containerconfig["cpulevel"],
			ramlevel:   containerconfig["ramlevel"],
			timetolive: containerconfig["timetolive"],
		},
	)
}

// Selects best container according to alert.
func (ss *Supervisor) selectcontainer(alert int) (candidateid int) {
	candidatescore := 0

	switch alert {
	case 1: // cpu
		for i, elem := range ss.containers {
			if candidatescore < elem.cores {
				candidatescore = elem.cores
				candidateid = i
			}
		}
	case 2: // ram
		for i, elem := range ss.containers {
			if candidatescore < elem.memory {
				candidatescore = elem.memory
				candidateid = i
			}
		}
	case 3: // combined
		for i, elem := range ss.containers {
			if aux := elem.cores * elem.memory; candidatescore < aux {
				candidatescore = elem.cores
				candidateid = i
			}
		}
	}
	return
}

// Triggers migration threshold alert.
func (ss *Supervisor) triggeralert() {
	var alert int

	for {
		if ss.cpulevel > ss.cputhreshold {
			if ss.ramlevel > ss.ramthreshold {
				alert = 3 // both
			} else {
				alert = 1 // cpu only
			}
		} else if ss.ramlevel > ss.ramthreshold {
			alert = 2 // ram only
		} else {
			alert = 0 // normal status
		}

		if alert > 0 {
			err := ss.migratecontainer(alert) // blocks method until container is migrated ignoring alarms in the meantime
			if err != nil {
				fmt.Println("Migration error", err)
			} else {
				alert = 0
			}
		}

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Starts migration of container using the WBP protocol.
func (ss *Supervisor) migratecontainer(alert int) error {
	// select best container for migration according to alert
	containerid := ss.selectcontainer(alert)

	// get servers in network
	conn, err := net.Dial("tcp", ss.manageraddress)
	if err != nil {
		fmt.Println("Connection error", err)
		return err
	}
	defer conn.Close()
	encoder := gob.NewEncoder(conn)
	p := &message.Packet{message.ServerList, `{"id":` + strconv.Itoa(ss.id) + `}`}
	encoder.Encode(p)

	dec := gob.NewDecoder(conn)
	p = &message.Packet{}
	dec.Decode(p)

	// call for proposals
	candidates := make([]map[string]interface{})
	serveraddresslist := strings.Split(p.Data[1:len(p.Data)])
	for _, serveraddress := range serveraddresslist {
		serverconn, err := net.Dial("tcp", serveraddress)
		if err != nil {
			fmt.Println("Connection error", err)
			return err
		}

		encoder := gob.NewEncoder(serverconn)
		containerconfig := fmt.Sprint(`{"cores":`, strconv.Itoa(ss.containers[containerid].cores), `,"memory":`, strconv.Itoa(ss.containers[containerid].memory), `}`)
		p := &message.Packet{message.CallForProposals, containerconfig}
		encoder.Encode(p)

		dec := gob.NewDecoder(serverconn)
		p = &message.Packet{}
		dec.Decode(p)
		if p.Msgtype == message.Proposal {
			candidates = append(candidates, message.Decodepacket(p)) //snapshot of candidate server
		}
		serverconn.Close()
	}

	// pick best candidate
	migrationdone := false
	for migrationdone == false {
		bcid := 0
		switch alert {
		case 1: // pick the server with the lowest CPU usage
			cpulevel := 100
			for i, candidate := range candidates {
				if candidate["cpulevel"] < cpulevel {
					bcid = i
				}
			}
		case 2: // pick the server with the lowest RAM usage
			ramlevel := 100
			for i, candidate := range candidates {
				if candidate["ramlevel"] < ramlevel {
					bcid = i
				}
			}
		case 3: // pick the server with the lowest combined usage
			combinedlevel := 10000
			for i, candidate := range candidates {
				if (candidate["cpulevel"] * candidate["ramlevel"]) < combinedlevel {
					bcid = i
				}
			}
		}

		// confirm best candidate can still host the container
		candidateconn, err := net.Dial("tcp", candidates[bcid]["address"]+":"+candidates[bcid]["port"])
		if err != nil {
			fmt.Println("Connection error", err)
			return err
		}
		defer candidateconn.Close()

		encoder = gob.NewEncoder(candidateconn)
		containerconfig := fmt.Sprint(`{"cores":`, strconv.Itoa(ss.containers[containerid].cores), `,"memory":`, strconv.Itoa(ss.containers[containerid].memory), `}`)
		p = &message.Packet{message.Accepted, containerconfig}
		encoder.Encode(p)

		dec = gob.NewDecoder(candidateconn)
		p = &message.Packet{}
		dec.Decode(p)

		// candidate is able to host
		if p.Msgtype == message.Accepted {
			// migrate container
			p = &message.Packet{message.Migration, containerconfig}
			encoder.Encode(p)
			// inform migration completed
			p = &message.Packet{message.MigrationDone, "Done!"}
			encoder.Encode(p)
			ss.containers[containerid].timetolive == 0
			migrationdone == true
		} else {
			log.Println("Migration cancelled.")
		}
	}
}

// Updates the time to live of every container.
func (ss *Supervisor) updatecontainers() {
	for i, container := range ss.containers {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		container.cpulevel += rand.Intn(7) - 2
		rand.Seed(time.Now().UnixNano()) // independent variables
		container.ramlevel += rand.Intn(5) - 1

		if container.cpulevel < 0 {
			container.cpulevel = 0
		} else if container.cpulevel > 100 {
			log.Printf("Container %d from server %d exceeded CPU limit (100%).", i)
			container.timetolive = 0
		}

		if container.ramlevel < 0 {
			container.ramlevel = 0
		} else if container.ramlevel > 100 {
			log.Printf("Container %d from server %d exceeded RAM limit (100%).", i)
			container.timetolive = 0
		}

		container.timetolive -= container.timetolive
	}
}

// Updates each container's time to live.
func (ss *Supervisor) checkexpiredcontainers() {
	for {
		ss.updatecontainers()
		alivecontainers := make([]Containers, len(ss.containers))
		for i, container := range ss.containers {
			if ss.containers[i].timetolive > 0 {
				alivecontainers := append(alivecontainers, container)
			}
		}
		ss.containers = alivecontainers

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Checks if the server can host a proposed container.
func (ss *Supervisor) canhost(containerconfig []map[string]interface{}) (canhost bool) {
	canhost = false
	coresused := 0
	memoryused := 0
	for _, container := range ss.containers {
		coresused += container.cores
		memoryused += container.memory
	}
	if (coresused+containerconfig["cores"]) <= ss.cores && (memoryused+containerconfig["memory"]) <= ss.memory {
		canhost = true
	}
	return
}

// Updates server resource usage and sends snapshot to CM.
func (ss *Supervisor) updatesupervisorstatus() {
	cpu, ram := 0
	for i, _ := range ss.containers {
		cpu += ss.containers[i][cpulevel]
		ram += ss.containers[i][ramlevel]
	}
	ss.cpulevel = (cpu / len(containers)).(int)
	ss.ramlevel = (ram / len(containers)).(int)
}

// Handles inputs and routes them to the corresponding functions of the Server Supervisor (SS).
func (ss *Supervisor) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)
	defer conn.Close()

	// decoding JSON string
	datajson := message.Decodepacket(*p)

	exitcode := 0
	remoteaddress := conn.RemoteAddr().String()
	switch p.Msgtype {
	case message.ContainerAllocation: // container allocated by Center Manager (CM)
		ss.addcontainer(datajson)
	case message.ServerUsage: // resource usage request from Center Manager (CM)
		jsonmap, err := json.Marshal(ss)
		if err != nil {
			log.Printf("Converting supervisor to JSON failed.")
		} else {
			sssnapshot := string(jsonmap)
			exitcode = message.Send(message.Packet{message.ServerUsage, sssnapshot}, remoteaddress, 8080) // responds to CM
			if exitcode != 0 {
				log.Println("Server ", ss.id, " snapshot delivery failed.")
			}
		}
	case message.WBP: // Workload Balancing Protocol (call for proposals, accept/reject proposal or successful/failed migration)
		wbptype := datajson["type"]
		if wbptype == message.CallForProposals {
			if ss.canhost(datajson) {
				// send snapshot
				jsonmap, err := json.Marshal(ss)
				if err != nil {
					log.Printf("Converting supervisor %d to JSON failed.", ss.id)
				} else {
					sssnapshot := string(jsonmap)
					encoder = gob.NewEncoder(conn)
					p = &message.Packet{message.Proposal, sssnapshot}
					encoder.Encode(p)
				}
			} else {
				// send refusal
				encoder = gob.NewEncoder(conn)
				p = &message.Packet{message.Rejected, ""}
				encoder.Encode(p)
			}
		} else if wbptype == message.Accepted {
			if ss.canhost(datajson) {
				// send confirmation
				encoder = gob.NewEncoder(conn)
				p = &message.Packet{message.Acepted, ""}
				encoder.Encode(p)

				// wait for container and add it to ss
				dec = gob.NewDecoder(conn)
				p = &message.Packet{}
				dec.Decode(p)
				container := message.Decodepacket(p)
				ss.addcontainer(container)

				// wait for inform-done
				dec = gob.NewDecoder(conn)
				p = &message.Packet{}
				dec.Decode(p)

				if wbptype == message.MigrationDone {
					log.Println("Container %d migrated from %s to %s:%i.", container["id"], remoteaddress, ss.address, ss.port)
				}
			} else {
				//  send cancelation
				encoder = gob.NewEncoder(conn)
				p = &message.Packet{message.Rejected, ""}
				encoder.Encode(p)
			}
		}
	}
}

// Sets up handling for incoming connections and starts monitoring goroutines.
func (ss *Supervisor) Run() {
	log.Println("Starting Server Supervisor (SS)")

	// monitoring methods
	go ss.updatesupervisorstatus()
	go ss.checkexpiredcontainers()
	go ss.triggeralert()

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(ss.port))
	if err != nil {
		log.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			log.Println("Connection error", err)
			continue
		}
		go ss.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}
}

// Executes the Server Supervisor (SS).
func main() {
	// this launches a new SS with the specified configuration and type.
	ss := NewSupervisor(0, 6, 16, "localhost", 8081, 0)
	ss.Run()
}
