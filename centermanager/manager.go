package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"github.com/juanjosedemiguel/loadbalancingsim/serversupervisor"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

type Centermanager struct {
	cpulevels, ramlevels []int
	serverlist           []Serversupervisor // stores server information
}

// Constructs a new SS.
func NewCentermanager(cores, ram, cpulevel, ramlevel int, servertype MsgType) *Centermanager {
	cm := Serversupervisor{
		cpulevels:  []int{4},
		ramlevels:  []int{4},
		serverlist: []Serversupervisor{},
	}

	// startup serversupervisors?

	return &cm
}

// Allocates a new container to a server (SS) to increase it's load.
func (cm *Centermanager) increaseload(serveraddress string, containerconfig string) int {
	return message.Send(message.Packet{2, containerconfig}, serveraddress, 8081)
}

// Deallocates (stops) a container from a server (SS) to decrease it's load.
func (cm *Centermanager) decreaseload(serveraddress string, containerid string) int {
	return message.Send(message.Packet{2, containerid}, serveraddress, 8081)
}

// Requests resource usage information from a Server Supervisor (SS).
func (cm *Centermanager) requestresources(serverid int, serveraddress string) {
	exitcode := message.Send(message.Packet{3, ""}, serveraddress, 8081)
	if exitcode == 0 {
		p := message.Listen(8080)
		splitdata := strings.Split(",", p.Data)
		serverusage[serverid] = splitdata[0] + "," + splitdata[1]
		log.Println("Resources from server ", serverid, "(", serveraddress, "): CPU[", splitdata[0], "%], RAM[", splitdata[1], "%]")
		conn.Close()
		log.Printf("%v", serverusage)
	} else {
		log.Println("Server ", serveraddress, " is unavailable for requests.")
	}
}

// Gathers resource usage information from all servers (SS).
func (cm *Centermanager) checkcenterstatus() {
	for i, element := range serverlist {
		go cm.requestresources(i, element)
	}
	time.Sleep(time.Duration(updateinterval) * time.Millisecond) // wait for next resource usage update
}

// Handles inputs from the Task Administrator (TA) and routes them to the corresponding functions of the Center Manager (CM).
func (cm *Centermanager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)

	datajson = message.Decodepacket(p)
	var exitcode int
	// container request from TA
	switch p.Msgtype {
	case 1: // add or remove container
		var serveraddress string
		requesttype := datajson["ip"]
		if requesttype == "none" {
			log.Println("Increase load (add container)")
			rand.Seed(time.Now().UnixNano())                       // different seed for every iteration
			serveraddress = serverlist[rand.Intn(len(serverlist))] // uniformfly distributed container allocations
			containerconfig := fmt.Sprintf(`{"cores":%i,"ram":%i}`, datajson["cores"].(int), datajson["ram"].(int))
			exitcode = cm.increaseload(serveraddress, containerconfig)
		} else {
			log.Println("Decrease load (remove container)")
			containerid := fmt.Sprintf(`{"containerid":%i}`, datajson["containerid"].(int))
			exitcode = cm.decreaseload(datajson["ip"].(string), containerid)
		}

		if exitcode == 0 {
			log.Println("Received : ", p.Data)
		} else {
			log.Println("Server ", serveraddress, " is unavailable for requests.")
		}
	case 3: // server usage information received. (PENDING)
		//
	}
}

//
func (cm *Centermanager) Run {
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
		go cm.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}

	go cm.checkcenterstatus()
}

// Sets up system - loads servers (PENDING) and handling for incoming connections
func main() {
	cm := NewCentermanager()
	cm.Run(8081)
}
