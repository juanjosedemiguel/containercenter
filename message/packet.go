package message

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type Packet struct {
	Msgtype MsgType
	Data    string
}

type MsgType uint8

const (
	Tipo0 MsgType = iota // = 0 (compute intensive)
	Tipo1                // = 1 (memory intensive)
	Tipo2                // = 2 (combined)
)

// Sends a message that consists of a Packet struct to a specified server.
func Send(packet Packet, serveraddress string, port int) (exitcode int) {
	exitcode = 0
	conn, err := net.Dial("tcp", serveraddress+strconv.Itoa(port))
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

// Listens for incoming packets on a specified port.
func Listen(port int) (p Packet) {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Connection error", err)
	}
	conn, err := ln.Accept() // this blocks until connection or error
	if err != nil {
		fmt.Println("Connection error", err)
	}
	dec := gob.NewDecoder(conn)
	dec.Decode(&p)
	defer conn.Close()
	return
}

// Returns payload from packet in a JSON map.
func Decodepacket(p Packet) (data map[string]interface{}) {
	byt := []byte(p.Data)

	if err := json.Unmarshal(byt, &data); err != nil {
		fmt.Println("Broken JSON/packet.")
		panic(err)
	}
	return
}
