package Manager

import (
	"encoding/gob"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"github.com/juanjosedemiguel/loadbalancingsim/server"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"
)

type Manager struct {
	serverlist []server.Supervisor // Supervisor instances involved in the simulation
	serverinfo []map[string]interface{}
}

// Constructs a new Center Manager.
func NewManager(servertype message.MsgType) *Manager {
	manager := Manager{
		serverlist: []server.Supervisor{},
		serverinfo: []map[string]interface{}{},
	}

	return &manager
}

// Allocates a new container to a server (SS) to increase it's load.
func (manager *Manager) increaseload(serveraddress string, containerconfig string) int {
	return message.Send(message.Packet{2, containerconfig}, serveraddress, 8081)
}

// Deallocates (stops) a container from a server (SS) to decrease it's load.
func (manager *Manager) decreaseload(serveraddress string, containerid string) int {
	return message.Send(message.Packet{2, containerid}, serveraddress, 8081)
}

// Requests resource usage information from a Server Supervisor (SS).
func (manager *Manager) requestresources(serverid int, serveraddress string) {
	exitcode := message.Send(message.Packet{3, ""}, serveraddress, 8081)
	if exitcode == 0 {
		log.Println("Server ", serveraddress, " received request.")
	} else {
		log.Println("Server ", serveraddress, " is unavailable for requests.")
	}
}

// Gathers resource usage information from all servers (SS).
func (manager *Manager) checkcenterstatus() {
	for i, element := range manager.serverinfo {
		go manager.requestresources(i, element["id"].(string))
	}
	time.Sleep(time.Duration(1000) * time.Millisecond) // wait for next resource usage update
}

// Handles inputs from the Task Administrator (TA) and routes them to the corresponding functions of the Center Manager (CM).
func (manager *Manager) handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)

	datajson := message.Decodepacket(*p)

	var exitcode int
	// container request from TA
	switch p.Msgtype {
	case 1: // add or remove container
		var serveraddress string
		requesttype := datajson["ip"]
		if requesttype == "none" {
			log.Println("Increase load (add container)")
			rand.Seed(time.Now().UnixNano())                                                           // different seed for every iteration
			serveraddress = manager.serverinfo[rand.Intn(len(manager.serverinfo))]["address"].(string) // uniformfly distributed container allocations
			containerconfig := fmt.Sprintf(`{"cores":%i,"ram":%i}`, datajson["cores"].(int), datajson["ram"].(int))
			exitcode = manager.increaseload(serveraddress, containerconfig)
		} else {
			log.Println("Decrease load (remove container)")
			containerid := fmt.Sprintf(`{"containerid":%i}`, datajson["containerid"].(int))
			exitcode = manager.decreaseload(datajson["ip"].(string), containerid)
		}

		if exitcode == 0 {
			log.Println("Received : ", p.Data)
		} else {
			log.Println("Server ", serveraddress, " is unavailable for requests.")
		}
	case 3: // server usage information received.
		sid := datajson["id"].(int)
		manager.serverinfo[sid] = datajson
	}
}

// Sets up handling for incoming connections.
func (manager *Manager) Run(port int) {
	log.Println("Starting Center Manager (CM)")
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			log.Println("Connection error", err)
			continue
		}
		go manager.handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}

	go manager.checkcenterstatus()
}

// Sets up system - loads servers and handling for incoming connections
func main() {

	// Starts the Center Manager
	manager := NewManager(message.Tipo0)
	go manager.Run(8080)

	// Starts the Server Supervisors of the simulation
	// 32 cores, 128 GB of RAM (one of each server types)
	cmd := fmt.Sprint("lxc-info -n c", containerid)
	out, err := exec.Command(cmd).Output() // (PENDING)
	if err != nil {
		log.Fatal(err) // (PENDING)
	}
	cmdoutput := string(out)
}
