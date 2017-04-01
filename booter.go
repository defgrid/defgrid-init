package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"os"
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

		// Environment variables can be used to customize how we
		// fake various aspects of the system.
		consoleDev := os.Getenv("DGI_DEV_CONSOLE")
		if consoleDev == "" {
			consoleDev = "/dev/null"
		}

		logDev := os.Getenv("DGI_DEV_LOG")
		if logDev == "" {
			logDev = "/dev/tty"
		}

		return &Booter{
			consoleDevPath:      consoleDev,
			logDevPath:          logDev,
			randomConfig:        &RandomConfigurerNoOp{},
			networkConfig:       &NetworkConfigurerLocalDev{},
			earlyResolverConfig: &ResolverConfigurerNoOp{},
			nodeConfigGetter:    &NodeConfigGetterLocalDev{},
			resolverConfig:      &ResolverConfigurerNoOp{},
		}

	case "devcontainer":
		// "devcontainer" is similar to "dev" except that we expect to be
		// launching in some sort of container, such as a Docker container.
		// This means it is somewhat isolated from the host system but still
		// isn't controlling a full machine and so ends up being a mixture
		// of "dev" and "testhost" config.

		// Environment variables can be used to customize how we
		// fake various aspects of the system.
		consoleDev := os.Getenv("DGI_DEV_CONSOLE")
		if consoleDev == "" {
			consoleDev = "/dev/null"
		}

		logDev := os.Getenv("DGI_DEV_LOG")
		if logDev == "" {
			logDev = "/dev/tty"
		}

		return &Booter{
			consoleDevPath: consoleDev,
			logDevPath:     logDev,
			randomConfig:   &RandomConfigurerNoOp{},
			networkConfig: &NetworkConfigurerLocalDev{
				ForceInterface: "eth0", // assume docker container with preconfigured eth0
			},
			earlyResolverConfig: &ResolverConfigurerNoOp{},
			nodeConfigGetter:    &NodeConfigGetterLocalDev{},
			resolverConfig:      &ResolverConfigurerNoOp{},
		}

	case "testhost":
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
			consoleDevPath:      "/dev/tty1",
			logDevPath:          "/dev/hvc0", // virtio console
			randomConfig:        &RandomConfigurerHaveged{},
			networkConfig:       &NetworkConfigurerDHCP{Interface: "eth0"},
			earlyResolverConfig: &ResolverConfigurerResolvDirect{},
			nodeConfigGetter:    &NodeConfigGetterTestNet{},

			// TODO: Once we've got consul running, write implementation
			// that configures dnsmasq to forward .consul requests over
			// to the DNS interface on the local consul agent.
			resolverConfig: &ResolverConfigurerResolvDirect{},
			///resolverConfig:      &ResolverConfigurerResolvWithConsul{},
		}
	}

	return nil
}

type Booter struct {
	consoleDevPath      string
	logDevPath          string
	randomConfig        RandomConfigurer
	networkConfig       NetworkConfigurer
	earlyResolverConfig ResolverConfigurer
	nodeConfigGetter    NodeConfigGetter
	resolverConfig      ResolverConfigurer

	earlyResolverActive bool
}

func (b *Booter) Console() (*Console, error) {
	return OpenConsole(b.consoleDevPath)
}

func (b *Booter) LogWriter() (io.WriteCloser, error) {
	return os.OpenFile(b.logDevPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
}

func (b *Booter) ConfigurePRNG() error {
	return b.randomConfig.ConfigurePRNG()
}

func (b *Booter) GenerateHostKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err == nil {
		key.Precompute()
	}
	return key, err
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
