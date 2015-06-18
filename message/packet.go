package message

import (
	"encoding/gob"
	"fmt"
	"net"
)

type Packet struct {
	Msgtype MsgType
	Data    interface{}
}

type MsgType uint8

const (
	ContainerRequest MsgType = iota
	ContainerAllocation
	ServerUsage
	ServerList
	CallForProposals
	Proposal
	Accepted
	Rejected
	Migration
	MigrationDone
)

type ServerType uint8

const (
	HighCPU    ServerType = iota // = 0 (compute intensive)
	HighMemory                   // = 1 (memory intensive)
	Combined                     // = 2 (combined)
)

// Sends a message that consists of a Packet struct to a specified server.
func Send(packet Packet, serveraddress string) (exitcode int) {
	exitcode = 0
	conn, err := net.Dial("tcp", serveraddress)
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
