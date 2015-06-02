package main

import (
	"encoding/gob"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"log"
	"net"
	"time"
)

func runtask() int {
	return 0
}

// Poisson process that simulates arrivals of VM requests.
func main() {
	fmt.Println("Start Task Administrator (TA)")

	for i := 0; i < 10; i++ {
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			log.Fatal("Connection error", err)
		}
		encoder := gob.NewEncoder(conn)
		p := &message.Packet{1, "2,8"} // number of cores and memory in GB
		encoder.Encode(p)
		fmt.Println("i: ", i)
		time.Sleep(1000 * time.Millisecond)
		conn.Close()
	}

	fmt.Println("Done.")
}
