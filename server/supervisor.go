package server

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Container struct {
	cores, memory      int
	cpulevel, ramlevel int
	lastusage          float64
}

type Supervisor struct {
	address                               string
	servertype                            message.MsgType
	id, cores, memory, cpulevel, ramlevel int
	cputhreshold, ramthreshold            int
	intervalstart                         time.Time
	containers                            []Container
}

// Constructs a new SS.
func NewSupervisor(id, cores, memory, cpulevel, ramlevel int, servertype message.MsgType) *Supervisor {
	ss := Supervisor{
		address:       address,
		cores:         cores,
		memory:        memory,
		cpulevel:      cpulevel,
		ramlevel:      ramlevel,
		servertype:    message.Tipo0,
		containers:    []Container{},
		intervalstart: time.Now(),
	}

	switch servertype {
	case 0:
		ss.cputhreshold = 50
		ss.ramthreshold = 10
	case 1:
		ss.cputhreshold = 10
		ss.ramthreshold = 50
	case 2:
		ss.cputhreshold = 30
		ss.ramthreshold = 30
	}
	return &ss
}

// Adds a container in the lxc hypervisor with an assigned configuration (by the TA). (PENDING)
func (ss Supervisor) addcontainer(containerconfig map[string]interface{}) int {
	return 0
}

// Removes specified container in the lxc hypervisor. (PENDING)
func (ss *Supervisor) removecontainer(containerid int) int {
	return 0
}

// Reads container operational status.
func (ss *Supervisor) readcontainerstatus(containerid int) {
	cmd := fmt.Sprint("lxc-info -n c", containerid)
	out, err := exec.Command(cmd).Output() // (PENDING)
	if err != nil {
		log.Fatal(err) // (PENDING)
	}
	cmdoutput := string(out)
	lines := strings.Split(cmdoutput, "\n")

	var rawcpu, rawram float64
	if rawcpu, err = strconv.ParseFloat(strings.Fields(lines[4])[2], 64); err != nil {
		log.Fatal(err)
	}
	if rawram, err = strconv.ParseFloat(strings.Fields(lines[6])[2], 64); err != nil {
		log.Fatal(err)
	}

	fmt.Println("CPU: ", rawcpu, " seconds")
	fmt.Println("RAM: ", rawram, " MiB")

	// convert raw values to percentages
	cpu := ((rawcpu - ss.containers[containerid].lastusage) / (time.Now().Sub(ss.intervalstart).Seconds())) / 100
	ram := (rawram / float64(ss.containers[containerid].memory)) / 100

	// update container info
	ss.containers[containerid].cpulevel = int(cpu)
	ss.containers[containerid].ramlevel = int(ram)
	ss.containers[containerid].lastusage = rawcpu
}

// Checks server resource usage and updates (PENDING).
func (ss *Supervisor) checklxcstatus() {
	for i, _ := range ss.containers {
		go ss.readcontainerstatus(i) // goroutines?
	}
	ss.intervalstart = time.Now()

	// send snapshot to CM
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
	// containerconfig := fmt.Sprint(`{"cores":`, container.cores, `, "ram":`, container.ram, `}`)
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

// Handles inputs and routes them to the corresponding functions of the Server Supervisor (SS).
func (ss *Supervisor) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)

	// decoding JSON string
	byt := []byte(p.Data)
	var datajson map[string]interface{}
	if err := json.Unmarshal(byt, &datajson); err != nil {
		fmt.Println("Broken JSON/packet.")
		panic(err)
	}

	var exitcode int
	var serveraddress = conn.RemoteAddr().String()
	switch p.Msgtype {
	case 2: // container addition/removal request from Center Manager (CM)
		fmt.Println("Received : ", datajson)

		if val, ok := datajson["containerid"]; ok { // decrease load
			ss.removecontainer(val.(int))
		} else { // increase load
			ss.addcontainer(datajson)
		}
	case 3: // CPU/RAM usage request from Center Manager (CM)
		jsonmap, err := json.Marshal(ss)
		if err != nil {
			log.Printf("Broken JSON string.")
		} else {
			fmt.Println(string(jsonmap))
			sssnapshot := fmt.Sprint(string(jsonmap))
			exitcode = message.Send(message.Packet{3, sssnapshot}, serveraddress, 8080) // responds to CM
		}
	case 4: // Workload Balancing Protocol (call for proposals, accept/reject proposal or successful/failed migration)
		wbptype := datajson["type"]
		if wbptype == "call" {
		} else if wbptype == "proposal" {
		} else if wbptype == "response" {
		} else if wbptype == "confirmation" {
		} else if wbptype == "termination" {
		}
	}
	if exitcode == 0 {
		fmt.Println("Connection error.")
	} else {
		fmt.Println("Server ", serveraddress, " is unavailable for requests.")
	}
}

// Sets up handling for incoming connections.
func (ss *Supervisor) Run(port int) {
	fmt.Println("Starting Server Supervisor (SS)")
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			fmt.Println("Connection error", err)
			continue
		}
		go ss.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}
}

// Executes the Server Supervisor (SS).
func main() {

	// this launches a new SS with the specified configuration and type.
	ss := NewSupervisor(0, 6, 16, 2, 2, 0)
	ss.Run(8081)
}
