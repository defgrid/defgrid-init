package main

import (
	"fmt"
)

// NodeConfigGetterLocalDev is a NodeConfigGetter implementation that just
// synthesizes some reasonable NodeConfig values based on the given network
// config.
//
// This is intended primarily for local dev environments, where we just need
// something valid enough to get through the boot process without interfering
// with the host system.
type NodeConfigGetterLocalDev struct {
}

func (n *NodeConfigGetterLocalDev) GetNodeConfig(net *NetworkConfig) (*NodeConfig, error) {
	ipAddr := net.IPAddress
	// Here we're assuming an IPv4 address, since that's all we support
	// right now.
	hostname := fmt.Sprintf(
		"ip-%02x%02x%02x%02x",
		ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3],
	)
	return &NodeConfig{
		Hostname:       hostname,
		RegionName:     "local-dev",
		DatacenterName: "local-dev",
	}, nil
}
