package main

import (
	"fmt"
	"net"

	"github.com/d2g/dhcp4"
	"github.com/d2g/dhcp4client"
	"github.com/milosgajdos83/tenus"
)

type Client struct {
	iface     *net.Interface
	ctrl      tenus.Linker
	rawClient *dhcp4client.Client
}

// NewClient returns a new client ready to control the named interface.
//
// An error is returned if the given interface either doesn't exist or
// it can't be controlled for some reason. Unlike errors when we're trying
// to get a lease, these errors are likely to be fatal and so this operation
// is not worth retrying without e.g. a change to system configuration or
// user permissions.
func NewClient(interfaceName string) (*Client, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}

	ctrl, err := tenus.NewLinkFrom(interfaceName)
	if err != nil {
		return nil, err
	}

	// We need to bring the interface up if it isn't already, so we can
	// create a raw packet socket for it.
	//
	// In the unlikely event that it's already up we'll bring it down
	// first. We don't care about any errors here because we expect this
	// to fail in the normal case.
	ctrl.SetLinkDown()

	err = ctrl.SetLinkUp()
	if err != nil {
		return nil, err
	}

	conn, err := dhcp4client.NewPacketSock(iface.Index)
	if err != nil {
		return nil, err
	}

	macAddr := iface.HardwareAddr

	rawClient, err := dhcp4client.New(
		dhcp4client.HardwareAddr(macAddr),
		dhcp4client.Connection(conn),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		iface:     iface,
		ctrl:      ctrl,
		rawClient: rawClient,
	}, nil
}

// Request sends a request to the DHCP server for a lease. If oldLease is
// not nil, it is treated as an earlier lease that the caller wishes to
// renew. Otherwise, an entirely new lease is requested.
//
// If an error is returned from this function it is generally best to pause
// briefly and then retry.
func (c *Client) Request(oldLease *Lease) (*Lease, error) {
	var success bool
	var ackPacket dhcp4.Packet
	var err error

	if oldLease == nil {
		success, ackPacket, err = c.rawClient.Request()
	} else {
		success, ackPacket, err = c.rawClient.Renew(oldLease.rawPacket)
	}

	if err != nil {
		return nil, err
	}

	if !success {
		return nil, fmt.Errorf("request unsuccessful")
	}

	return newLease(ackPacket)
}

func (c *Client) ConfigureInterface(lease *Lease) error {
	network := &net.IPNet{
		IP:   lease.IPAddress,
		Mask: lease.SubnetMask,
	}
	err := c.ctrl.SetLinkIp(lease.IPAddress, network)
	if err != nil {
		return err
	}

	if len(lease.Routers) > 0 {
		err := c.ctrl.SetLinkDefaultGw(&lease.Routers[0])
		if err != nil {
			return fmt.Errorf(
				"set IP address but failed to configure default gateway: %s", err,
			)
		}
	}

	return nil
}
