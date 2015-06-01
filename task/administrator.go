package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"message"
	"net"
)

func runtask() int {

}

// Poisson pŕocess that simulates arrivals of VM requests.
func main() {
	fmt.Println("Start Task Administrator (TA)")
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal("Connection error", err)
	}
	encoder := gob.NewEncoder(conn)
	for i := 0; i < 10; i++ {
		p := &Packet{1, "cores:2,Memorysize:8, i:" += i}
		encoder.Encode(p)
	}
	conn.Close()
	fmt.Println("Done")
}
