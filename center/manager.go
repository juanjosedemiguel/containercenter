package main

import (
	"encoding/gob"
	"fmt"
	"message"
	"net"
)

var (
	serverlist []string
)


// Inform server.supervisor to start executing a container.
func addcontainer(serveraddress, containerconfig string) {
	conn, err := net.Dial("tcp",  serveraddress += ":8080")
	if err != nil {
		log.Fatal("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &Packet{2, containerconfig}
	encoder.Encode(p)
	conn.Close()
}

// Inform server.supervisor to stop executing a container.
func removecontainer(serveraddress, containerid string) {
	conn, err := net.Dial("tcp", serveraddress += ":8080")
	if err != nil {
		log.Fatal("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &Packet{2, containerid}
	encoder.Encode(p)
	conn.Close()
}

// Request resource usage information from a server.supervisor.
// Waits until it receives an answer and then exits.
func requestresources(serveraddress s) {
	conn, err := net.Dial("tcp", serveraddress += ":8080")
	if err != nil {
		log.Fatal("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	p := &Packet{3, ""}
	encoder.Encode(p)
	conn.Close()

	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		// handle error
	}
	conn, err := ln.Accept() // this blocks until connection or error
	if err != nil {
		fmt.Printf("Error when connecting to server: %+v", serveraddress)
		continue
	}
	dec := gob.NewDecoder(conn)
	p := &Packet{}
	dec.Decode(p)
	fmt.Printf("Resources from server (%+v): %+v", serveraddress, p.message)
	conn.Close()
}

// Gather resource usage information from all servers.
func checkcenterstatus() {
	// for list of servers requestresources(server)
	for  index, element := range serverlist {
		go requestresources(element)
	}
}

// Handle inputs through the socket.
func handleConnection(conn net.Conn) {
	dec := gob.NewDecoder(conn)
	p := &Packet{}
	dec.Decode(p)

	switch p.msgtype {
	case 1:
		// container request from task.administrator
		fmt.Printf("Received : %+v", p)
	case 3:
		// CPU/RAM usage from server.supervisor
		fmt.Printf("Received : %+v", p)
	}
}

// Sets up system: load servers and listening
func main() {
	fmt.Println("start")
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		// handle error
	}
	for {
		conn, err := ln.Accept() // this blocks until connection or error
		if err != nil {
			// handle error
			continue
		}
		go handleConnection(conn) // a goroutine handles conn so that the loop can accept other connections
	}
}
