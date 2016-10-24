package main

import (
	"io"
	"log"
	"net"
	"os"
	"os/exec"

	"gopkg.in/vmihailenco/msgpack.v2"
)

// NetworkConfigurerDHCP is a NetworkConfigurer implementation that
// obtains a DHCP lease and configures the network stack based on that.
//
// In order to keep renewing the DHCP lease, the caller *must* keep
// calling ConfigureNetwork in the background. After the first call,
// ConfigureNetwork will block until the lease is renewed.
type NetworkConfigurerDHCP struct {
	Interface string

	leaseDecoder *msgpack.Decoder
}

func (cer *NetworkConfigurerDHCP) ConfigureNetwork() (*NetworkConfig, error) {

	// On the first call we'll launch the DHCP client as a child
	// process, and then we'll monitor it via subsequent calls.
	if cer.leaseDecoder == nil {
		leaseRead, leaseWrite := io.Pipe()
		cmd := exec.Command("/usr/lib/defgrid-init/dhcpclient", "eth0")
		cmd.Stdout = leaseWrite
		cmd.Stderr = os.Stderr

		go func() {
			err := cmd.Run()
			if err != nil {
				log.Printf("[ERROR] DHCP client failed: %s", err)
				os.Exit(1)
			} else {
				log.Printf("[WARNING] DHCP client has exited")
				os.Exit(0)
			}
		}()

		cer.leaseDecoder = msgpack.NewDecoder(leaseRead)
	}

	lease := &networkConfigurerDHCPLease{}
	err := cer.leaseDecoder.Decode(&lease)
	if err != nil {
		return nil, err
	}

	return &NetworkConfig{
		IPAddress:  lease.IPAddress,
		SubnetMask: lease.SubnetMask,
		Routers:    lease.Routers,

		SuggestedHostname:    lease.Hostname,
		SuggestedDomainName:  lease.DomainName,
		SuggestedNameservers: lease.Nameservers,
	}, nil
}

type networkConfigurerDHCPLease struct {
	IPAddress   net.IP     `msgpack:"ip_address"`
	Hostname    string     `msgpack:"hostname"`
	DomainName  string     `msgpack:"domain_name"`
	SubnetMask  net.IPMask `msgpack:"subnet_mask"`
	Routers     []net.IP   `msgpack:"routers"`
	Nameservers []net.IP   `msgpack:"name_servers"`
}
