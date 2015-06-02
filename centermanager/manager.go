package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"math/rand"
	"net"
	"strings"
	"time"
)

var (
	updateinterval int      // resource usage updates
	serverlist     []string // stores server information in the format: {"156.22.30.66", "40.56.120.10"}
	serverusage    []string // stores cpu and ram precentages in the format: {"55,37", "26,94"}
)

// Allocates a new container to a server (SS) to increase it's load.
func increaseload(serveraddress string, containerconfig string) int {
	return message.Send(message.Packet{2, containerconfig}, serveraddress, 8080)
}

// Deallocates (stops) a container from a server (SS) to decrease it's load.
func decreaseload(serveraddress string, containerid string) int {
	return message.Send(message.Packet{2, containerid}, serveraddress, 8080)
}

// Requests resource usage information from a Server Supervisor (SS).
func requestresources(serverid int, serveraddress string) {
	exitcode := message.Send(message.Packet{3, ""}, serveraddress, 8080)
	if exitcode == 0 {
		ln, err := net.Listen("tcp", ":8080")
		if err != nil {
			fmt.Println("Connection error", err)
		}
		conn, err := ln.Accept() // this blocks (it's own thread) until connection or error
		if err != nil {
			fmt.Println("Connection error", err)
		}
		dec := gob.NewDecoder(conn)
		p := &message.Packet{}
		dec.Decode(p)
		splitdata := strings.Split(",", p.Data)
		serverusage[serverid] = splitdata[0] + "," + splitdata[1]
		fmt.Println("Resources from server ", serverid, "(", serveraddress, "): CPU[", splitdata[0], "%], RAM[", splitdata[1], "%]")
		conn.Close()
		fmt.Printf("%v", serverusage)
	} else {
		fmt.Println("Server ", serveraddress, " is unavailable for requests.")
	}
}

// Gathers resource usage information from all servers (SS).
func checkcenterstatus() {
	for i, element := range serverlist {
		go requestresources(i, element)
	}
	time.Sleep(time.Duration(updateinterval) * time.Millisecond) // wait for next resource usage update
}

// Handles inputs from the Task Administrator (TA) and routes them to the corresponding functions of the Center Manager (CM).
func handleConnection(conn net.Conn) {
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
	// container request from TA
	switch p.Msgtype {
	case 1: // add or remove container
		var serveraddress string
		requesttype := datajson["ip"]
		if requesttype == "none" {
			fmt.Println("Increase load (add container)")
			rand.Seed(time.Now().UnixNano())                       // different seed for every iteration
			serveraddress = serverlist[rand.Intn(len(serverlist))] // uniformfly distributed container allocations
			containerconfig := fmt.Sprintf(`{"cores":%i,"ram":%i}`, datajson["cores"].(int), datajson["ram"].(int))
			exitcode = increaseload(serveraddress, containerconfig)
		} else {
			fmt.Println("Decrease load (remove container)")
			containerid := fmt.Sprintf(`{"containerid":%i}`, datajson["containerid"].(int))
			exitcode = decreaseload(datajson["ip"].(string), containerid)
		}

		if exitcode == 0 {
			fmt.Println("Received : ", p.Data)
		} else {
			fmt.Println("Server ", serveraddress, " is unavailable for requests.")
		}
	case 3: // Server usage information received.

	}
}

// Sets up system - loads servers (MISSING) and handling for incoming connections
func main() {
	fmt.Println("Starting Center Manager (CM)")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Connection error", err)
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			fmt.Println("Connection error", err)
			continue
		}
		go handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}

	go checkcenterstatus()
}
