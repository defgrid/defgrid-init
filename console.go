// -*- coding: utf-8 -*-
package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Console manages the system's local display.
//
// When the system is working correctly a user should have no reason to look
// at the local display, but it is provided as a last-resort mechanism for
// debugging should SSH access and the serial log become inoperative.
type Console struct {
	tty *os.File

	// Must be held before writing to "tty", to ensure that concurrent
	// writers don't try to do conflicting operations with escape sequences
	// that could corrupt the display.
	writeMutex sync.Mutex

	// If BootStatusMessage is non-empty, the console will show a
	// a full-screen boot logo with the status message beneath it.
	// This is used during early boot when basic system information
	// is not yet known.
	//
	// Transitioning from the normal status screen back to the boot
	// logo will lose any log information and is not suggested.
	BootStatusMessage string

	// The remaining fields are used only when BootStatusMessage
	// is empty.

	SystemRoleName string
	SystemRoleIcon ConsoleIcon
	Services       []ConsoleService
	IPAddress      net.IP
	Hostname       string
	RegionName     string

	logPreserved bool

	// Set if someone calls FatalError, in which case we'll render a big
	// ugly red error on the console instead of the usual status output.
	fatalError error
}

type ConsoleService struct {
	Icon   ConsoleIcon
	Status ServiceStatus
}

type ConsoleIcon int

const (
	ConsoleIconNone ConsoleIcon = iota
	ConsoleIconConsul
	ConsoleIconVault
	ConsoleIconNomad
	ConsoleIconBastion
	ConsoleIconTunnel
	ConsoleIconPrometheus
	ConsoleIconBootstrap
)

var consoleIconBytes map[ConsoleIcon][]byte
var consoleLogoSmallBytes []byte
var consoleBootLogoIndentBytes []byte
var consoleRuleBytes []byte
var consoleStatusColors map[ServiceStatus]int

func init() {
	// We use some codepoints in the unicode BMP private use area
	// to show some service icons and our logo on the console,
	// via our custom console font. This will render as garbage
	// in a dev environment that isn't actually running on a
	// text-mode console with our font loaded, but that's okay.

	consoleIconBytes = map[ConsoleIcon][]byte{
		ConsoleIconConsul: []byte("\uef00\uef01"),
		ConsoleIconVault:  []byte("\uef02\uef03"),
	}

	consoleLogoSmallBytes = []byte("\ue000\ue001\ue002\ue003\ue004\ue005\ue006\ue007\ue000\ue008\ue009\ue000\ue001")

	consoleBootLogoIndentBytes = make([]byte, 40-(56/2))
	for i, _ := range consoleBootLogoIndentBytes {
		consoleBootLogoIndentBytes[i] = 32 // space
	}

	consoleRuleBytes = []byte("────────────────────────────────────────────────────────────────────────────────")

	consoleStatusColors = map[ServiceStatus]int{
		ServiceCritical: 31,
		ServiceWarning:  33,
		ServicePassing:  32,
	}
}

func OpenConsole(devPath string) (*Console, error) {
	tty, err := os.OpenFile(devPath, os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	c := &Console{tty: tty}
	c.BootStatusMessage = "Initializing..."
	c.setFont(devPath)
	c.Refresh()

	return c, nil
}

func (c *Console) Refresh() {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	if c.fatalError != nil {
		c.displayFatalError()
		return
	}

	if c.BootStatusMessage != "" {
		c.displayBootStatus()
	} else {
		c.displayRuntimeStatus()
	}
}

func (c *Console) Logf(format string, args ...interface{}) {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if c.BootStatusMessage != "" || c.fatalError != nil {
		// Can't show log lines while we're in the boot logo or error states.
		return
	}

	// If we're currently in a state where the log area hasn't been
	// initialized, we'll draw the overall screen layout first and
	// then this message will be the first one in the log.
	if !c.logPreserved {
		c.displayRuntimeStatus()
	}

	// Newline gets written first so that we leave the cursor
	// on the end of the most recent line, and thus avoid a blank
	// line at the end of the log area.
	c.tty.Write([]byte{10})
	fmt.Fprintf(c.tty, format, args...)
}

func (c *Console) FatalError(err error) {
	c.fatalError = err
	c.Refresh()
}

func (c *Console) displayBootStatus() {
	c.logPreserved = false
	c.clearAndReset()

	c.tty.Write([]byte("\033[1;35m\033[10;0H"))

	r := strings.NewReader(consoleBootLogo)
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		c.tty.Write(consoleBootLogoIndentBytes)
		c.tty.Write(lines.Bytes())
		c.tty.Write([]byte("\n"))
	}

	statusLen := len(c.BootStatusMessage)
	col := 40 - (statusLen / 2)
	c.tty.Write(
		[]byte(fmt.Sprintf(
			"\033[18;%dH\033[1;37m%s\n",
			col, c.BootStatusMessage,
		)),
	)
}

func (c *Console) displayRuntimeStatus() {
	resetScrollRegion := false
	if !c.logPreserved {
		c.clearAndReset()
		resetScrollRegion = true
	}
	c.logPreserved = true

	if resetScrollRegion {
		// Horizontal rules
		c.tty.Write(
			[]byte("\033[1;0H\033[1;37m"),
		)
		c.tty.Write(consoleRuleBytes)
		c.tty.Write(
			[]byte("\033[7;0H\033[1;30m"),
		)
		c.tty.Write(consoleRuleBytes)
		c.tty.Write(
			[]byte("\033[25;0H\033[1;30m"),
		)
		c.tty.Write(consoleRuleBytes)
		c.tty.Write(
			[]byte("\033[1;0H\033[1;37m "),
		)

		// Title bar
		if c.SystemRoleIcon != ConsoleIconNone {
			iconBytes := consoleIconBytes[c.SystemRoleIcon]
			if iconBytes != nil {
				c.tty.Write(iconBytes)
				c.tty.Write([]byte{32})
			}
		}
		c.tty.Write([]byte(c.SystemRoleName))
		c.tty.Write([]byte{32})

		// Small Defgrid logo
		c.tty.Write(
			[]byte("\033[1;66H\033[1;35m\033[K "),
		)
		c.tty.Write(consoleLogoSmallBytes)
	} else {
		// Save the current terminal state so we can restore it
		// in a moment once we're don refreshing the status area.
		c.tty.Write([]byte("\0337"))
	}

	// Basic system information
	// (The second line here also clears out the area with the
	// service icons, so we don't need to worry about re-erasing that
	// below.)
	fmt.Fprintf(c.tty, "\033[3;5H\033[0;37m\033[KIP Address: %s", c.IPAddress)
	fmt.Fprintf(c.tty, "\033[4;5H\033[0;37m\033[KHostname:   %s", c.Hostname)
	fmt.Fprintf(c.tty, "\033[5;5H\033[0;37m\033[KRegion:     %s", c.RegionName)

	// Service icons
	// Each icon takes up two character cells and we include a space
	// before each one for a total of three.
	if c.Services != nil && len(c.Services) != 0 {
		serviceIconsWidth := len(c.Services) * 3
		fmt.Fprintf(c.tty, "\033[4;%dH", 77-serviceIconsWidth)
		for _, service := range c.Services {
			iconBytes := consoleIconBytes[service.Icon]
			statusColor := consoleStatusColors[service.Status]
			if iconBytes == nil {
				iconBytes = []byte{32, 32}
			}
			fmt.Fprintf(c.tty, "\033[1;%dm", statusColor)
			c.tty.Write([]byte{32})
			c.tty.Write(iconBytes)
		}
	}

	if resetScrollRegion {
		// Either we're drawing for the first time or something erased
		// the screen, so we need to do initial config of the log scrolling
		// area and leave the cursor at the bottom of this area so that
		// any new log lines will scroll up into view.
		c.tty.Write([]byte("\033[8;24r\033[24;0H\033[0;37m"))
	} else {
		// Just restore the terminal state from before we refreshed
		// the status area.
		c.tty.Write([]byte("\0338"))
	}
}

func (c *Console) displayFatalError() {
	// The fatal error box is designed to fit into the same space as the
	// status pane at the top of the runtime status UI while still showing
	// the log below, though it can also render over the top of the boot
	// logo, which will look a little funny but we don't care too much.
	c.tty.Write(
		[]byte("\033[0;31m\033[1;0H"),
	)

	bigBlock := []byte("\u2588")
	smallBlockTop := []byte("\u2580")
	smallBlockBottom := []byte("\u2584")

	c.tty.Write(bigBlock)
	for i := 0; i < 78; i++ {
		c.tty.Write(smallBlockTop)
	}
	c.tty.Write(bigBlock)

	for y := 2; y < 5; y++ {
		fmt.Fprintf(c.tty, "\033[%d;0H\033[K", y)
		c.tty.Write(bigBlock)
		fmt.Fprintf(c.tty, "\033[%d;80H", y)
		c.tty.Write(bigBlock)
	}

	c.tty.Write([]byte("\033[5;0H"))
	c.tty.Write(bigBlock)
	for i := 0; i < 78; i++ {
		c.tty.Write(smallBlockBottom)
	}
	c.tty.Write(bigBlock)

	errMsg := c.fatalError.Error()
	if len(errMsg) > 76 {
		errMsg = errMsg[0:73] + "..."
	}

	fmt.Fprintf(c.tty, "\033[2;3HCritical System Error. See system log for more details.")
	fmt.Fprintf(c.tty, "\033[4;3H%s", errMsg)
}

func (c *Console) clearAndReset() {
	// Reset terminal + mode, clear screen, move to 0,0 and enable UTF-8 mode.
	// Then unblank the screen and configure it to only blank after a long time.
	// Cursor is invisible.
	c.tty.Write(
		[]byte("\033c\033[l\033[0m\033[2J\033[0;0H\033%G\033[9;60]\033[13]\033[?25l"),
	)
}

func (c *Console) setFont(dev string) {
	// We will try our best to set a font here, but we won't sweat
	// it if we can't, since the console output is unimportant.
	setFont, err := exec.LookPath("setfont")
	if err != nil {
		return
	}

	cmd := exec.Command(setFont, "-C", dev, "/usr/share/consolefonts/defgrid.psf")
	cmd.Run()
}

// io.Writer implementation that writes lines it recieves to
// the console log. Partial lines are buffered until a newline
// is received.
type consoleLogWriter struct {
	pipe    io.Writer
	console *Console
}

func (c *Console) LogWriter() io.Writer {
	r, w := io.Pipe()

	// Watch the "read" end of this pipe, which will
	// get data each time new data is written to the logger.
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			c.Logf("%s", scanner.Text())
		}
	}()

	return &consoleLogWriter{
		pipe:    w,
		console: c,
	}
}

func (w *consoleLogWriter) Write(buf []byte) (int, error) {
	return w.pipe.Write(buf)
}

var consoleBootLogo = `█████▄▖ ███     ███████  ▗███▖  ██████▖ ████ █████▄▖
███████ ███     ███████ ▗█████  ███████ ████ ███████
███████ █████   ███████ █████▘  ███████ ████ ███████
███████ █████   █████   ████▘   ███████ ████ ███████
███████ ███████ █████   ███████ █████▖  ████ ███████
███████ ███████ ███     ▝██████ ██████▖ ████ ███████
█████▀▘ ███████ ███      ▝███▀█ ███████▖████ █████▀▘
`
