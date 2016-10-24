package main

import (
	"fmt"
	"log"
	"os"
)

// ResolverConfigurerResolvDirect is a ResolverConfigurer implementation that
// just writes the DHCP-provided nameservers directly into /etc/resolv.conf .
//
// This is intended for use as an "early config", before Consul is available.
// It's not suitable for use as the main configuration because it does not
// wire things up properly so that the .consul domain is routed into
// Consul's DNS interface.
type ResolverConfigurerResolvDirect struct {
}

func (r *ResolverConfigurerResolvDirect) ConfigureResolver(net *NetworkConfig, node *NodeConfig) error {
	f, err := os.Create("/etc/resolv.conf")
	if err != nil {
		return err
	}

	if net.SuggestedNameservers == nil || len(net.SuggestedNameservers) == 0 {
		return nil
	}

	log.Println("Configuring /etc/resolv.conf...")
	for _, nsIP := range net.SuggestedNameservers {
		log.Printf("resolv.conf nameserver %s", nsIP)
		_, err := fmt.Fprintf(f, "nameserver %s\n", nsIP)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ResolverConfigurerResolvDirect) UnconfigureResolver() error {
	log.Println("Removing /etc/resolv.conf...")
	err := os.Remove("/etc/resolv.conf")
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
