package main

import (
	"encoding/gob"
	"fmt"
	"github.com/juanjosedemiguel/loadbalancingsim/message"
	"log"
	"math/rand"
	"net"
	"strconv"
	"time"
)

func runtask() int {
	return 0
}

// Poisson process that simulates arrivals of VM requests.
func main() {
	fmt.Println("Starting Task Administrator (TA)")

	for i := 0; i < 30; i++ {
		rand.Seed(time.Now().UnixNano()) // different seed for every iteration
		cores := rand.Intn(8) + 1        // number of cores allowed to the container [1-8]
		ram := rand.Intn(30) + 2         // ram [2-32 GB]
		requestjson := `{"ip":"none","cores":` + strconv.Itoa(cores) + `,"ram":` + strconv.Itoa(ram) + `}`
		fmt.Println(cores)
		fmt.Println(ram)
		fmt.Println(requestjson)
		message.Send(message.Packet{1, requestjson}, "localhost", 8080)
		fmt.Println("i: ", i)
		time.Sleep(1000 * time.Millisecond)
	}

	fmt.Println("Done.")
}
