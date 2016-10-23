package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/d2g/dhcp4client"
	"github.com/milosgajdos83/tenus"
	"gopkg.in/vmihailenco/msgpack.v2"
)

func main() {
	// Right now this is just a hard-coded prototype of getting
	// a DHCP lease and configuring the network, which will
	// be the first thing this program does but ultimately it
	// will do it in a more robust and organized way.

	// Make sure we never exit, even on panic, because if we do
	// that we'll cause a kernel panic that will obscure our abililty
	// to see the panic output.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from %s", r)
		}
		log.Printf("That's all I've got, folks.")
		for {
			time.Sleep(60 * time.Second)
		}
	}()

	leaseRead, leaseWrite := io.Pipe()
	cmd := exec.Command("/usr/lib/defgrid-init/dhcpclient", "eth0")
	cmd.Stdout = leaseWrite
	cmd.Stderr = os.Stderr

	go func() {
		err := cmd.Run()
		if err != nil {
			log.Printf("[ERROR] %s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	leaseDecoder := msgpack.NewDecoder(leaseRead)

	lease := map[string]interface{}{}
	for {
		err := leaseDecoder.Decode(&lease)
		if err != nil {
			log.Printf("[ERROR] failed to parse DHCP lease notification: %s", err)
			continue
		}

		log.Printf("Got DHCP lease %#v", lease)
	}
}
