package main

import (
	"log"
	"net"
	"os"

	"github.com/milosgajdos83/tenus"
	"github.com/d2g/dhcp4client"
)

func main() {
	// Right now this is just a hard-coded prototype of getting
	// a DHCP lease and configuring the network, which will
	// be the first thing this program does but ultimately it
	// will do it in a more robust and organized way.
	//
	// In particular, this is not yet ready to be used as a real
	// /sbin/init because it exits once it's fussed with DHCP.

	log.Printf("Arguments are %#v", os.Args)

	link, err := tenus.NewLinkFrom("eth0")
	if err != nil {
		panic(err)
	}
	macAddr := link.NetInterface().HardwareAddr

	log.Printf("MAC address is %s", macAddr)

	log.Println("requesting DHCP lease")

	c, err := dhcp4client.NewInetSock(
		dhcp4client.SetLocalAddr(
			net.UDPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 68},
		),
		dhcp4client.SetRemoteAddr(
			net.UDPAddr{IP: net.IPv4bcast, Port: 67},
		),
	)
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
		IP: localIPAddr,
		Mask: subnetMask,
	}

	log.Printf("hostname is %q", hostname)
	log.Printf("domain name is %q", domainName)
	log.Printf("subnet mask is %s", subnetMask)
	log.Printf("routers are %#v", routers)
	log.Printf("name servers are %#v", nameServers)

	log.Printf("configuring eth0 (this might break SSH)")

	// Ignore error since we don't care if the interface is already down,
	// and if it's some other sort of error then we'll presumably get the
	// same error in a moment when we SetLinkUp
	link.SetLinkDown()

	err = link.SetLinkUp()
	if err != nil {
		panic(err)
	}

	err = link.SetLinkIp(localIPAddr, &network)
	if err != nil {
		panic(err)
	}

	err = link.SetLinkDefaultGw(&routers[0])
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
