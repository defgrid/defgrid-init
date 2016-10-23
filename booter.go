package main

import (
	"fmt"
)

func NewBooter(flavor string) *Booter {
	switch flavor {

	case "dev":
		// "dev" means just launching the program directly within a
		// standalone dev environment; this mode may not do anything
		// that requires root access or make any permanent changes
		// to the system, since the primary motivation is to get
		// through the boot process with little fanfare so we can
		// test the service-supervision part.
		return &Booter{
			networkConfig:       &NetworkConfigurerLocalDev{},
			earlyResolverConfig: &ResolverConfigurerNoOp{},
			nodeConfigGetter:    &NodeConfigGetterLocalDev{},
			resolverConfig:      &ResolverConfigurerNoOp{},
		}

		/*case "testhost":
		// "testhost" is another kind of dev environment, but used
		// when we're running in a local qemu instance launched from
		// within the defgrid-images repository. In this case we
		// *are* booting a virtual machine, and so we do need to go
		// through all the usual network configuration steps, but
		// there's no "metadata service" with which to discover our
		// node id and region, and so we'll just use synthetic
		// values for these which are designed to be "unique enough"
		// for our test network.
		return &Booter{
			networkConfig:       &NetworkConfigurerDHCP{Interface: "eth0"},
			earlyResolverConfig: &ResolverConfigurerResolvDirect{},
			nodeConfigGetter:    &NodeConfigGetterTestNet{},
			resolverConfig:      &ResolverConfigurerResolvWithConsul{},
		}
		*/
	}

	return nil
}

type Booter struct {
	networkConfig       NetworkConfigurer
	earlyResolverConfig ResolverConfigurer
	nodeConfigGetter    NodeConfigGetter
	resolverConfig      ResolverConfigurer

	earlyResolverActive bool
}

func (b *Booter) ConfigureNetwork() (*NetworkConfig, error) {
	return b.networkConfig.ConfigureNetwork()
}

func (b *Booter) EarlyConfigureResolver(c *NetworkConfig) error {
	b.earlyResolverActive = true
	return b.earlyResolverConfig.ConfigureResolver(c, nil)
}

func (b *Booter) GetNodeConfig(c *NetworkConfig) (*NodeConfig, error) {
	return b.nodeConfigGetter.GetNodeConfig(c)
}

func (b *Booter) ConfigureResolver(net *NetworkConfig, node *NodeConfig) error {
	if b.earlyResolverActive {
		err := b.earlyResolverConfig.UnconfigureResolver()
		if err != nil {
			return fmt.Errorf("failed to disable early resolver config: ", err)
		}
		b.earlyResolverActive = false
	}

	return b.resolverConfig.ConfigureResolver(net, node)
}
