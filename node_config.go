package main

// NodeConfig is the configuration of a particular node from the perspective
// of how it interacts with its hosting platform and with the rest of the
// defgrid infrastructure.
type NodeConfig struct {
	Hostname       string
	RegionName     string
	DatacenterName string
}

// NodeConfigGetter implementations discover the local node's configuration,
// usually by reaching out to some platform-specific configuration endpoint
// and then massaging the results to what makes sense within the defgrid
// architecture.
type NodeConfigGetter interface {

	// GetNodeConfig discovers and returns the local node configuration.
	// This is called once during boot and it's assumed that the information
	// returned will remain valid for the lifetime of the node.
	GetNodeConfig(*NetworkConfig) (*NodeConfig, error)
}
