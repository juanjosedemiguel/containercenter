package message

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
)

type Packet struct {
	var msgtype int
	var data string
}