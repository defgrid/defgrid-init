// dhcpclient is the DHCP client component of defgrid-init.
//
// It is launched as a child process of defgrid-init because the main
// init process is not permitted to interact directly with the network.
//
// It is not designed to be run as a standalone tool. It produces on
// its stdout a msgpack-encoded datastructure describing the network
// settings, and then stays running to renew the lease. If configuration
// changes when the lease is renewed, further data structures are produced
// on its stdout, so the caller should repeatedly execute blocking reads
// to efficiently watch for changes.
//
// It is guaranteed that by the time a configuration message is produced
// on stdout the configuration has already been applied to the local
// network interfaces. This client *only* handles the interface IP address,
// subnet mask and default gateway. It is the caller's responsibility
// to arrange for the system resolver to be configured for the returned
// DNS server settings. The hostname and domain name provided by the DHCP
// server are also returned, though in most cases defgrid-init will ignore
// these and instead use a platform-specific instance id as the hostname
// and the "node.<region>.consul" domain as the domain.
package main

import (
	"fmt"
	"gopkg.in/vmihailenco/msgpack.v2"
	"log"
	"os"
	"time"
)

func main() {

	if len(os.Args) != 2 {
		log.Println("usage: dhcpclient <interface-name>")
		os.Exit(1)
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] %s", r)
			os.Exit(2)
		} else {
			os.Exit(0)
		}
	}()

	ifaceName := os.Args[1]

	client, err := NewClient(ifaceName)
	if err != nil {
		panic(fmt.Errorf("can't open interface %s: %s", ifaceName, err))
	}

	var lease *Lease
	leaseEncoder := msgpack.NewEncoder(os.Stdout)

	for {
		lease, err := client.Request(lease)
		if err != nil {
			log.Printf("[ERROR] DHCP request failed: %s", err)
			// Don't make the DHCP server sweat
			time.Sleep(10)
			continue
		}

		err = client.ConfigureInterface(lease)
		if err != nil {
			log.Printf("[ERROR] %s configuration failed: %s", ifaceName, err)
			// This one's our problem, so retrying probably isn't going to
			// help. But rather than crash we will just retry occasionally.
			time.Sleep(60)
			continue
		}

		err = leaseEncoder.Encode(lease)
		if err != nil {
			// should never happen
			log.Printf("[WARN] Failed to encode lease for announcement: %s", err)
		}

		time.Sleep(lease.Duration)
	}
}
