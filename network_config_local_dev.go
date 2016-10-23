package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
)

// NetworkConfigurerLocalDev is a NetworkConfigurer implementation that
// does not actually alter the network configuration at all, and instead
// just returns the pre-existing network configuration for whatever
// interface is holding the default route.
//
// This implementation is useful only as a stub in a local dev environment,
// where we want to be able to run without root access and get just enough
// information to get through the boot process and test the supervisor part
// of defgrid-init.
type NetworkConfigurerLocalDev struct {
}

func (cer *NetworkConfigurerLocalDev) ConfigureNetwork() (*NetworkConfig, error) {

	// We're going to use the "ip route" command (tested only on Linux)
	// to ask for the route table entry that would get us to 255.0.0.0
	// and then assume that the interface used for that route is the
	// "primary" one for the sake of our work here.
	//
	// This is a bit shifty but should work well enough for the simple
	// dev environment case.

	cmd := exec.Command("ip", "route", "get", "255.0.0.0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to connect pipe to 'ip route': %s", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error launching 'ip route': %s", err)
	}

	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read 'ip route' output: %s", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("'ip route' failed: %s", err)
	}

	// Now we'll do some shaky-shaky "parsing" of the output!
	srcIndex := bytes.Index(output, []byte(" dev "))
	if srcIndex == -1 {
		return nil, fmt.Errorf("'ip route' output does not meet expectations")
	}

	// Position the start of our slice at the beginning of the interface name
	output = output[srcIndex+5:]

	endIndex := bytes.IndexAny(output, " \n")
	if endIndex == -1 {
		return nil, fmt.Errorf("'ip route' output does not meet expectations")
	}

	// Position the end of our slice at the end of the interface name
	output = output[:endIndex]

	// Now "output" should frame just the name of our "default route"
	// interface!
	ifaceName := string(output)

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to look up interface %q", ifaceName)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %q", ifaceName)
	}

	var ip net.IP
	var mask net.IPMask
	for _, addr := range addrs {
		var ipNet *net.IPNet
		var ok bool
		if ipNet, ok = addr.(*net.IPNet); !ok {
			continue
		}

		_, len := ipNet.Mask.Size()
		if len != 32 {
			continue
		}

		ip = ipNet.IP.To4()
		mask = ipNet.Mask
		break
	}

	if ip == nil {
		return nil, fmt.Errorf("%q has no IPv4 addresses", ifaceName)
	}

	return &NetworkConfig{
		IPAddress:  ip,
		SubnetMask: mask,
		Routers:    []net.IP{ip},

		SuggestedNameservers: []net.IP{},
	}, nil
}
