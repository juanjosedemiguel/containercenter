package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"net"
	"os/exec"
	"strconv"
	"time"
)

var updateinterval int // resource usage updates

type Serversupervisor struct {
	cores, ram, cpulevel, ramlevel         int
	servertype, cputhreshold, ramthreshold int
	containers                             []string
}

// Constructs a new SS.
func NewServersupervisor(cores, ram, cpulevel, ramlevel, servertype int) *Serversupervisor {
	ss := Serversupervisor{
		cores:      64,
		ram:        256,
		cpulevel:   10,
		ramlevel:   10,
		servertype: 1,
		containers: []string{},
	}
	// compute intensive (1), memory intensive (2), combined (3)
	switch servertype {
	case 1:
		ss.cputhreshold = 50
		ss.ramthreshold = 10
	case 2:
		ss.cputhreshold = 10
		ss.ramthreshold = 50
	case 3:
		ss.cputhreshold = 30
		ss.ramthreshold = 30
	}
	return &ss
}

// Updates the server resource usage attributes (cpu, ram). (PENDING)
func (ss *Serversupervisor) setserverresources() {

}

// Updates the containerlist attribute. (PENDING)
func (ss *Serversupervisor) setserverstatus() {

}

// Adds a container in the lxc hypervisor with an assigned configuration (by the TA). (PENDING)
func (ss Serversupervisor) addcontainer(containerconfig map[string]interface{}) int {
	return 0
}

// Removes specified container in the lxc hypervisor. (PENDING)
func (ss *Serversupervisor) removecontainer(containerid int) int {
	return 0
}

// Reads container operational status.
func (ss *Serversupervisor) readcontainerstatus(containerid int) {
	cmd := exec.Command("lxc") // (PENDING)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

// Checks server resource usage and updates (PENDING).
func (ss *Serversupervisor) checklxdstatus() {
	for i, _ := range ss.containers {
		go ss.readcontainerstatus(i)
	}
	time.Sleep(time.Duration(updateinterval) * time.Millisecond) // wait for next resource usage update
}

// Triggers migration threshold alert.
func (ss *Serversupervisor) triggeralert() {
	var alert int

	for {
		if cpulevel > cputhreshold {
			if ramlevel > ramthreshold {
				alert = 3 // both
			} else {
				alert = 1 // cpu only
			}
		} else if ramlevel > ramthreshold {
			alert = 2 // ram only
		}
		alert = 0 // normal status

		if alert > 0 {
			ss.migratecontainer(alert) // blocks method until container is migrated ignoring alarms in the meantime
		}

		time.Sleep(1000 * time.Millisecond)
	}
}

// Starts migration of container using WBP.
func (ss *Serversupervisor) migratecontainer(alert int) {
	// select best container for migration according to alert

	// send call for proposals

	// wait for proposals

	// choose best candidate

	// send proposal acceptance to best candidate

	// wait for confirmation (in case the candidate is no longer able to host)

}

// Handles inputs and routes them to the corresponding functions of the Server Supervisor (SS).
func (ss *Serversupervisor) handleConnection(conn net.Conn) {
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
		jsonslice, _ := json.Marshal(ss)
		fmt.Println(string(jsonslice))
		sssnapshot := fmt.Sprint(string(jsonslice))
		exitcode = message.Send(message.Packet{3, sssnapshot}, serveraddress, 8080) // responds to CM
	case 4: // Workload Balancing Protocol packet (container proposal)

	}
	if exitcode == 0 {
		fmt.Println("Connection error.")
	} else {
		fmt.Println("Server ", serveraddress, " is unavailable for requests.")
	}
}

// Sets up handling for incoming connections.
func (ss *Serversupervisor) run(port int) {
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

	go ss.checklxdstatus()
}

// Executes the Server Supervisor (SS) and launches the LXD daemon.
func main() {
	cmd := exec.Command("docker -H localhost -d -e lxc") // *hardcoded* CM address
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	ss := NewServersupervisor()
	ss.Run(8081)
}
