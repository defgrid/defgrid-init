package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/d2g/dhcp4"
)

type Lease struct {
	IPAddress   net.IP        `msgpack:"ip_address"`
	Hostname    string        `msgpack:"hostname,omitempty"`
	DomainName  string        `msgpack:"domain_name,omitempty"`
	SubnetMask  net.IPMask    `msgpack:"subnet_mask"`
	Routers     []net.IP      `msgpack:"routers"`
	NameServers []net.IP      `msgpack:"name_servers"`
	Duration    time.Duration `msgpack:"duration"`

	rawPacket dhcp4.Packet `msgpack:"-"`
}

func newLease(ackPacket dhcp4.Packet) (*Lease, error) {

	lease := &Lease{rawPacket: ackPacket}

	localIPAddr := ackPacket.YIAddr()
	options := ackPacket.ParseOptions()

	lease.IPAddress = localIPAddr

	if hostnameBytes := options[12]; hostnameBytes != nil {
		lease.Hostname = string(hostnameBytes)
	}

	if domainNameBytes := options[15]; domainNameBytes != nil {
		lease.DomainName = string(domainNameBytes)
	}

	if subnetBytes := options[1]; subnetBytes != nil && len(subnetBytes) == 4 {
		lease.SubnetMask = net.IPMask(subnetBytes)
	} else {
		return nil, fmt.Errorf("response has missing or malformed subnet mask")
	}

	if routerBytes := options[3]; routerBytes != nil {
		if len(routerBytes) == 0 || len(routerBytes)%4 != 0 {
			return nil, fmt.Errorf("response has missing or malformed router list")
		}

		routers := make([]net.IP, 0, len(routerBytes)/4)
		for i := 0; i < len(routerBytes); i = i + 4 {
			routers = append(routers, net.IP(routerBytes[i:i+4]))
		}
		lease.Routers = routers
	}

	if nameServerBytes := options[6]; nameServerBytes != nil {
		if len(nameServerBytes) == 0 || len(nameServerBytes)%4 != 0 {
			return nil, fmt.Errorf("response has missing or malformed nameserver list")
		}

		nameServers := make([]net.IP, 0, len(nameServerBytes)/4)
		for i := 0; i < len(nameServerBytes); i = i + 4 {
			nameServers = append(nameServers, net.IP(nameServerBytes[i:i+4]))
		}
		lease.NameServers = nameServers
	}

	if durationBytes := options[51]; durationBytes != nil && len(durationBytes) == 4 {
		lease.Duration = time.Duration(binary.BigEndian.Uint32(durationBytes)) * time.Second
	} else {
		// we'll just supply a reasonable default so we'll still check
		// in with the DHCP server every day.
		lease.Duration = 23 * time.Hour
	}

	return lease, nil
}
