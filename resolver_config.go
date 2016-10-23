package main

// ResolverConfigurer implementations configure the system resolver based
// on given network and node configurations.
type ResolverConfigurer interface {

	// ConfigureResolver configures the system resolver appropriately
	// before returning.
	//
	// NodeConfig can be nil for a configurer that is used as the
	// "early resolver configurer", responsible for getting a minimal
	// resolver configuration in place so we can reach out to the
	// network to discover the NodeConfig. The final configuration
	// will then be dealt with by the main configurer, which will
	// recieve valid instances for both arguments.
	ConfigureResolver(*NetworkConfig, *NodeConfig) error

	// UnconfigureResolver returns the system resolver to an
	// unconfigured state.
	//
	// This is currently used only for "early resolver configurers" so
	// that their work can be undone before we begin the final
	// configuration.
	UnconfigureResolver() error
}
