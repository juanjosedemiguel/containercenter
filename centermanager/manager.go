package main

import (
	"encoding/gob"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"net"
	"strings"
	"time"
)

var (
	updateinterval int      // resource usage updates
	serverlist     []string // stores server information in the format: {"156.22.30.66", "40.56.120.10"}
	serverusage    []string // stores cpu and ram precentages in the format: {"55,37", "26,94"}
)

// Inform server.supervisor to start executing a container.
func addcontainer(serveraddress, containerconfig string) {
	conn, err := net.Dial("tcp", serveraddress+":8080")
	if err != nil {
		fmt.Println("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &message.Packet{2, containerconfig}
	encoder.Encode(p)
	conn.Close()
}

// Inform server.supervisor to stop executing a container.
func removecontainer(serveraddress, containerid string) {
	conn, err := net.Dial("tcp", serveraddress+":8080")
	if err != nil {
		fmt.Println("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &message.Packet{2, containerid}
	encoder.Encode(p)
	conn.Close()
}

// Request resource usage information from a server.supervisor.
// Waits until it receives an answer and then exits.
func requestresources(serverindex int, serveraddress string) {
	conn, err := net.Dial("tcp", serveraddress+":8080")
	if err != nil {
		fmt.Println("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &message.Packet{3, ""}
	encoder.Encode(p)
	conn.Close()

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Connection error", err)
	}
	conn, err = ln.Accept() // this blocks until connection or error
	if err != nil {
		fmt.Println("Connection error", err)
	}
	dec := gob.NewDecoder(conn)
	p = &message.Packet{}
	dec.Decode(p)
	splitdata := strings.Split(",", p.Data)
	serverusage[serverindex] = splitdata[0] + "," + splitdata[1]
	fmt.Println("Resources from server ", serverindex, "(", serveraddress, "): CPU[", splitdata[0], "%], RAM[", splitdata[1], "%]")
	conn.Close()
	fmt.Printf("%v", serverusage)
}

// Gather resource usage information from all servers.
func checkcenterstatus() {
	// for list of servers requestresources(server)
	for i, element := range serverlist {
		go requestresources(i, element)
	}
	time.Sleep(time.Duration(updateinterval) * time.Millisecond) // wait for next resource usage update
}

// Handle inputs and route them to corresponding functions of the Center Manager (CM).
func handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &message.Packet{}
	dec.Decode(p)

	switch p.Msgtype {
	case 1:
		// container request from task.administrator
		fmt.Println("Received : ", p.Data)
	case 3:
		// CPU/RAM usage from server.supervisor
		fmt.Println("Received : ", p.Data)
	}
}

// Sets up system - load servers and connections
func main() {
	fmt.Println("Start")
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
