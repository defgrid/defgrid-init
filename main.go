package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"time"
)

func main() {
	// Trap if we return from this function, e.g. due to a panic, and
	// prevent the program from actually exiting so we don't panic
	// the kernel.
	panicHandler := PanicHandler{}
	defer panicHandler.OnExit()
	panicHandler.AddAction(func(err error, stack string) {
		log.Printf("[FATAL] %s\n%s", err, stack)
	})

	booter := NewBooter(os.Args[1])
	if booter == nil {
		panic(fmt.Errorf("unknown flavor %q", os.Args[1]))
	}

	console, err := booter.Console()
	if err != nil {
		panic(err)
	}

	logWriter, err := booter.LogWriter()
	if err != nil {
		panic(err)
	}

	console.Refresh()

	panicHandler.AddAction(func(err error, stack string) {
		console.FatalError(err)
	})

	log.SetOutput(io.MultiWriter(logWriter, console.LogWriter()))

	if booter == nil {
		panic("unsupported boot flavor!")
	}

	bootStatus := func(s string) {
		log.Println(s)
		console.BootStatusMessage = s
		console.Refresh()
	}

	bootStatus("Configuring network...")
	netConfig, err := booter.ConfigureNetwork()
	if err != nil {
		panic(err)
	}
	_, subnetPrefix := netConfig.SubnetMask.Size()
	log.Printf("Network Up: %s/%d", netConfig.IPAddress, subnetPrefix)

	bootStatus("Configuring system resolver...")

	err = booter.EarlyConfigureResolver(netConfig)
	if err != nil {
		panic(err)
	}

	bootStatus("Getting node configuration...")

	nodeConfig, err := booter.GetNodeConfig(netConfig)
	if err != nil {
		panic(err)
	}
	log.Printf("Node identity: %q, in region %q", nodeConfig.Hostname, nodeConfig.RegionName)

	bootStatus("Re-configuring system resolver...")

	err = booter.ConfigureResolver(netConfig, nodeConfig)
	if err != nil {
		panic(err)
	}

	console.BootStatusMessage = ""
	console.SystemRoleName = "Dev System"
	console.IPAddress = netConfig.IPAddress
	console.Hostname = nodeConfig.Hostname
	console.RegionName = nodeConfig.RegionName
	console.Refresh()

	// TODO: Eventually this will be our main event loop, but we
	// don't have any events right now so we'll just pause here
	// and do nothing.
	for {
		time.Sleep(3600)
	}
}

type PanicHandler struct {
	actions []PanicAction
}

type PanicAction func(err error, stack string)

func (h *PanicHandler) AddAction(action PanicAction) {
	h.actions = append(h.actions, action)
}

// OnExit should be called in a "defer" on the main function.
//
// If the main function ever exits, it will call the configured panic actions
// and then intentionally hang the program forever, thus preventing it from
// exiting.
//
// If we are exiting due to a panic, the panic error will be passed to the
// actions. Otherwise, the error will merely be that the program exited.
//
// Needless to say, we should only get in here if something has gone very
// wrong that prevents us from booting or continuing to operate the system.
// Wherever possible we should retry things and attempt to keep the system
// running.
func (h *PanicHandler) OnExit() {
	var err error
	var trace string
	if p := recover(); p != nil {
		trace = string(debug.Stack())

		switch tErr := p.(type) {
		case error:
			err = tErr
		case string:
			err = errors.New(tErr)
		case fmt.Stringer:
			err = errors.New(tErr.String())
		default:
			err = fmt.Errorf("panic: %#v", tErr)
		}
	} else {
		err = fmt.Errorf("main function exited")
	}

	actions := h.actions
	if actions == nil || len(actions) == 0 {
		// Default action if we're so early that we don't have any
		// actions yet.
		fmt.Fprintf(os.Stderr, "[FATAL] %s\n%s\n", err, trace)
	} else {
		for _, action := range actions {
			action(err, trace)
		}
	}

	// Hang out here forever
	for {
		time.Sleep(3600)
	}
}
