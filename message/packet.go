package message

import (
	"encoding/gob"
	"fmt"
	"net"
	"strconv"
)

type Packet struct {
	Msgtype int
	Data    string // mapa
}

// Sends a message that consists of a Packet struct to a specified server.
func Send(packet Packet, serveraddress string, port int) (exitcode int) {
	exitcode = 0
	conn, err := net.Dial("tcp", serveraddress+strconv.Itoa(port))
	if err != nil {
		fmt.Println("Connection error", err)
		exitcode = 1
	}
	encoder := gob.NewEncoder(conn)
	//p := &message.Packet{2, containerconfig}
	p := &packet
	encoder.Encode(p)
	conn.Close()
	return
}
