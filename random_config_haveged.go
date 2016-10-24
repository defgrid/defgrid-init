package main

import (
	"log"
	"os/exec"
)

// RandomConfigurerHaveged is a RandomConfigurer implementation that uses
// the application "haveged" to generate (hopefully-)random values using
// inconsistencies in the execution time of CPU instructions due to
// various details about the CPU's current state, such as contents of
// caches.
//
// Some sources have raised concerns about this approach, particularly when
// applied in shared-tenency virtual machine environments. This approach
// is generally used in defgrid on platforms where no superior solution is
// available, taking a "better than nothing" approach.
//
// Note that we use haveged not as a background daemon constantly feeding
// urandom but instead as a one-off bootstrapping entropy source to
// initialize the PRNG. It is assumed that once up and running a system
// will generate further entropy via ongoing I/O operations, etc.
type RandomConfigurerHaveged struct {
}

func (c *RandomConfigurerHaveged) ConfigurePRNG() error {
	log.Println("Initializing PRNG using haveged")
	// Generate 512 bytes to feed directly into /dev/urandom
	cmd := exec.Command("/usr/sbin/haveged", "-n", "4096", "-f", "/dev/urandom")
	return cmd.Run()
}
