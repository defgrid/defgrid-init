package main

import (
	"net"
)

type NetworkConfig struct {
	IPAddress  net.IP
	SubnetMask net.IPMask
	Routers    []net.IP

	// The NetworkConfig layer returns the resolver-related config
	// coming from the network if any, but the resolver configurer
	// may ignore this and use information obtained from elsewhere.
	//
	// The NetworkConfigurer does *not* take any action on these
	// resolver suggestions; it just returns verbatim what was
	// suggested by the network, if anything, as a hint or fallback
	// for later configuration.
	SuggestedHostname    string
	SuggestedDomainName  string
	SuggestedNameservers []net.IP
}

// NetworkConfigurer implementations obtain IP configuration from
// the host network and configure the local network stack to use it.
type NetworkConfigurer interface {

	// ConfigureNetwork sends a request for network configuration,
	// configures the network stack, and then returns a NetworkConfig
	// instance describing the configuration.
	//
	// This method blocks until network configuration can be obtained.
	// It should be called once during boot and then called repeatedly
	// in the background to await any changes to the configuration
	// caused by changing network conditions.
	//
	// Implementers of this interface should know that the defgrid system
	// assumes that a host's IP address will not change after initial
	// assignment, so if a subsequent NetworkConfig instance returns
	// a changed IP address this will result in undefined behavior; better
	// to return an error in this case, so the system can know it's
	// in a broken state.
	ConfigureNetwork() (*NetworkConfig, error)
}
