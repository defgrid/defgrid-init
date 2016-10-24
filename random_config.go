package main

// Implementations of RandomConfigurer will prepare the system's random
// number generator (/dev/random and /dev/urandom) for use on boot,
// e.g. by loading it with entropy.
//
// The purpose of RandomConfigurer is to deal with the issue that a
// freshly-booted machine is likely to have very little entropy in its
// PRNG entropy pool, but yet in early boot we need to generate
// crypto-secure private keys for host authentication.
//
// Finding good entropy in early boot of a virtual machine is challenging,
// so most implementations of this interface will be adopting a "better than
// nothing" approach, presuming that producing something that is somewhat
// challenging to predict is better than having no entropy at all. For
// the rare platform where an specialized hardware entropy source is
// available, this should be preferred over the pure-software implementations.
type RandomConfigurer interface {
	ConfigurePRNG() error
}
