package main

import (
	"log"
	"net"
	"os"

	"github.com/d2g/dhcp4client"
	"github.com/milosgajdos83/tenus"
)

func main() {
	// Right now this is just a hard-coded prototype of getting
	// a DHCP lease and configuring the network, which will
	// be the first thing this program does but ultimately it
	// will do it in a more robust and organized way.

	// Make sure we never exit, even on panic, because if we do
	// that we'll cause a kernel panic that will obscure our abililty
	// to see the panic output.
	defer func () {
		if r := recover(); r != nil {
			log.Printf("Recovered from %s", r)
		}
		log.Printf("That's all I've got, folks.")
		for {}
	}()

	log.Printf("Arguments are %#v", os.Args)

	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		panic(err)
	}
	log.Printf("eth0 index is %d", iface.Index)

	link, err := tenus.NewLinkFrom("eth0")
	if err != nil {
		panic(err)
	}
	macAddr := link.NetInterface().HardwareAddr

	log.Printf("MAC address is %s", macAddr)

	log.Println("requesting DHCP lease")

	// Ignore error since we don't care if the interface is already down,
	// and if it's some other sort of error then we'll presumably get the
	// same error in a moment when we SetLinkUp
	link.SetLinkDown()

	err = link.SetLinkUp()
	if err != nil {
		panic(err)
	}

	c, err := dhcp4client.NewPacketSock(iface.Index)
	if err != nil {
		panic(err)
	}

	client, err := dhcp4client.New(
		dhcp4client.HardwareAddr(macAddr),
		dhcp4client.Connection(c),
	)
	if err != nil {
		panic(err)
	}

	success, ackPacket, err := client.Request()
	if err != nil {
		panic(err)
	}

	log.Printf("Success %#v; Packet is %#v", success, ackPacket)

	if !success {
		return
	}

	localIPAddr := ackPacket.YIAddr()
	options := ackPacket.ParseOptions()

	log.Printf("Got IP address %s", localIPAddr)
	log.Printf("Got options %v", options)

	hostname := string(options[12])
	domainName := string(options[15])
	subnetMask := net.IPMask(options[1])
	routers := IPListFromDHCP(options[3])
	nameServers := IPListFromDHCP(options[6])
	network := net.IPNet{
		IP:   localIPAddr,
		Mask: subnetMask,
	}

	log.Printf("hostname is %q", hostname)
	log.Printf("domain name is %q", domainName)
	log.Printf("subnet mask is %s", subnetMask)
	log.Printf("routers are %#v", routers)
	log.Printf("name servers are %#v", nameServers)

	log.Printf("configuring eth0 (this might break SSH)")

	err = link.SetLinkIp(localIPAddr, &network)
	if err != nil {
		panic(err)
	}

	err = link.SetLinkDefaultGw(&routers[0])
	if err != nil {
		panic(err)
	}

}

// IPListFromDHCP takes a slice of IPv4 addresses expressed as raw
// octets obtained from DHCP options and creates a slice of net.IP
// instances.
//
// Since net.IP instances are themselves just byte slices, these
// new instances refer to offsets into the provided buffer.
//
// The raw array's length must be a multiple of 4, or else the
// function will panic.
func IPListFromDHCP(raw []byte) []net.IP {
	ret := make([]net.IP, 0, len(raw)/4)
	for i := 0; i < len(raw); i = i + 4 {
		ret = append(ret, net.IP(raw[i:i+4]))
	}
	return ret
}
