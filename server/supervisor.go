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
		id:         id,
		cores:      cores,
		memory:     memory,
		servertype: servertype,
		address:    address,
		port:       port,
		containers: []Container{},
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
			cores:      containerconfig["cores"],
			memory:     containerconfig["memory"],
			timetolive: containerconfig["timetolive"],
		},
	)
}

// Selects best container according to alert.
func (ss *Supervisor) selectcontainer(alert int) (candidateid int) {
	var candidatescore int

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
		}
		alert = 0 // normal status

		if alert > 0 {
			ss.migratecontainer(alert) // blocks method until container is migrated ignoring alarms in the meantime
		}

		time.Sleep(1000 * time.Millisecond) // check every second
	}
}

// Starts migration of container using the WBP protocol.
func (ss *Supervisor) migratecontainer(alert int) {
	// exitcode := 0

	// // select best container for migration according to alert
	// containerid := ss.selectcontainer(alert)
	// // send call for proposals
	// container := ss.containers[containerid]
	// containerconfig := log.Sprint(`{"cores":`, container.cores, `, "ram":`, container.ram, `}`)
	// // for every server send a proposal
	// exitcode = message.Send(message.Packet{4, containerconfig}, serveraddress, 8080)

	// // wait for proposals to arrive through channel (PENDING)
	// proposals = ss.getproposals()

	// // choose best candidate
	// candidateid := ss.getcandidate(proposals)

	// // send proposal acceptance to best candidate
	// exitcode = message.Send(message.Packet{4, `{"proposal": "accepted"`}, serveraddress, 8080)

	// // wait for confirmation (in case the candidate is no longer able to host)
	// packet = message.Listen(8081)
	// migration = message.Decodepacket(packet)["acceptance"]

	// // execute LXC command to migrate container
	// if migration == "confirmed" {
	// 	cmd := exec.Command("docker -H localhost -d -e lxc") // (PENDING)
	// 	err := cmd.Run()

	// 	// send migration notification
	// 	if err != nil {
	// 		panic(err)
	// 		exitcode = message.Send(message.Packet{4, `{"migration": "aborted"`}, serveraddress, 8080)
	// 	} else {
	// 		exitcode = message.Send(message.Packet{4, `{"migration": "successful"`}, serveraddress, 8080)
	// 	}
	// } else if migration == "canceled" {
	// 	log.Printf("Migration canceled by candidate.")
	// }
}

// Updates the time to live of every container.
func (ss *Supervisor) updatecontainers() {
	for _, container := range ss.containers {
		container.timetolive -= container.timetolive
	}
}

// Updates each container's time to live.
func (ss *Supervisor) checkexpiredcontainers() {
	ss.updatecontainers()
	alivecontainers := make([]Containers, len(ss.containers))

	for i, container := range ss.containers {
		if ss.containers[i].timetolive > 0 {
			alivecontainers := append(alivecontainers, container)
		}
	}
	ss.containers = alivecontainers
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
	// decoding JSON string
	datajson := message.Decodepacket(*p)

	exitcode := 0
	serveraddress := conn.RemoteAddr().String()
	switch p.Msgtype {
	case ContainerAllocation: // container allocated by Center Manager (CM)
		ss.addcontainer(datajson)

	case ServerUsage: // resource usage request from Center Manager (CM)
		jsonmap, err := json.Marshal(ss)
		if err != nil {
			log.Printf("Converting supervisor to JSON failed.")
		} else {
			ss.updatesupervisorstatus()
			sssnapshot := string(jsonmap)
			exitcode = message.Send(message.Packet{3, sssnapshot}, serveraddress, 8080) // responds to CM
			if exitcode != 0 {
				log.Println("Server ", ss.id, " snapshot delivery failed.")
			}
		}
	case WBP: // Workload Balancing Protocol (call for proposals, accept/reject proposal or successful/failed migration)
		wbptype := datajson["type"]
		if wbptype == "call" {
		} else if wbptype == "proposal" {
		} else if wbptype == "response" {
		} else if wbptype == "confirmation" {
		} else if wbptype == "termination" {
		}
	}
}

// Sets up handling for incoming connections.
func (ss *Supervisor) Run() {
	log.Println("Starting Server Supervisor (SS)")
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
