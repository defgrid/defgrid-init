package main

import (
	"log"
)

// RandomConfigurerNoOp is a RandomConfigurer implementation that does
// absolutely nothing.
//
// It is intended only for use in dev environments where the host system
// is assumed to already have a configured PRNG and where high-quality
// randomness is not necessary anyway.
type RandomConfigurerNoOp struct {
}

func (c *RandomConfigurerNoOp) ConfigurePRNG() error {
	log.Println("[WARNING] Assuming already-configured PRNG; taking no action")
	return nil
}
