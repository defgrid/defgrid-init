package main

// ResolverConfigurerNoOp is a ResolverConfigurer implementation that
// does absolutely nothing.
//
// It is intended for use in local dev environments where we assume there
// is already a working resolver configuration in place which we will
// use.
type ResolverConfigurerNoOp struct {
}

func (r *ResolverConfigurerNoOp) ConfigureResolver(*NetworkConfig, *NodeConfig) error {
	return nil
}

func (r *ResolverConfigurerNoOp) UnconfigureResolver() error {
	return nil
}
