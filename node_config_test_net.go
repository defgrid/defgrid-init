package main

import (
	"fmt"
)

// NodeConfigGetterTestNet is a NodeConfigGetter implementation that just
// synthesizes some reasonable NodeConfig values based on the given network
// config.
//
// This is intended for use in the test network configuration used within
// the defgrid-images repo. For now it looks pretty similar to the "local dev"
// config but is likely to grow more complex as the defgrid-images test
// setup grows to include multiple types of server, etc.
type NodeConfigGetterTestNet struct {
}

func (n *NodeConfigGetterTestNet) GetNodeConfig(net *NetworkConfig) (*NodeConfig, error) {
	ipAddr := net.IPAddress
	// Here we're assuming an IPv4 address, since that's all we support
	// right now.
	hostname := fmt.Sprintf(
		"ip-%02x%02x%02x%02x",
		ipAddr[0], ipAddr[1], ipAddr[2], ipAddr[3],
	)
	return &NodeConfig{
		Hostname:       hostname,
		RegionName:     "dgtest0",
		DatacenterName: "dgtest0a",
	}, nil
}
