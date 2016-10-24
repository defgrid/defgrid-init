package main

// This package implements defgrid's custom SSH server.
//
// Defgrid has its own SSH server to allow a direct integration with
// Vault for one-time-password access and to allow other custom behaviors
// that support the specific SSH workflow Defgrid requires.
//
// This SSH server is intended to be used only within the internal
// network. The Defgrid bastion image has its own SSH server that is
// intended to face the Internet and provide only TCP tunneling to
// other hosts.
//
// In a standard Defgrid deployment, security groups ensure that
// SSH is accessible only from "admin hosts", which includes bastion
// servers and bootstrap servers. This then avoids using other Internet-
// facing services as intermediaries to attack internal SSH endpoints.
//
// This server operates in two distinct modes, which are selected by the
// username used to authenticate:
//
// - "provisioning" is used for access by automated provisioning tools
//   and is authenticated using the publickey auth method against a
//   keypair assigned to the server at boot. Only very specific system
//   bootstrapping operations are permitted for this user, and this
//   user may not obtain an interactive terminal session nor invoke
//   the shell non-interactively.
//
// - "admin" is used for interactive access by humans and is
//   authenticated using one-time passwords issued by Vault. This
//   user may start an interactive shell and may open TCP tunnels
//   to arbitrary ports on the loopback interface only.
//   (this per-host SSH server is not intended to be used as a
//   "bounce host" to access other servers, and so it does not support
//   TCP tunnelling to other hosts. The separate Bastion SSH server
//   can be used to create tunnels to hosts for admin purposes.)
//

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"github.com/kr/pty"

	"golang.org/x/crypto/ssh"
)

func main() {
	// This is in a prototype state right now. Still fleshing this out
	// before deciding how best to structure it.

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			// This is where we'll eventually send the password off to
			// Vault to see if it is a valid one-time password that was
			// issued. For now, we just allow any password.
			if conn.User() == "admin" {
				// No special permissions
				return nil, nil
			}
			return nil, fmt.Errorf("invalid credentials")
		},
	}

	hostKey, err := rsa.GenerateKey(rand.Reader, 4095)
	if err != nil {
		// TODO: Do something better
		panic(err)
	}

	signer, err := ssh.NewSignerFromSigner(hostKey)
	if err != nil {
		// TODO: Do something better
		panic(err)
	}

	config.AddHostKey(signer)

	// Listening on port 4022 while we're prototyping, since it's easier
	// to run on a system that already has a more conventional sshd running.
	listener, err := net.Listen("tcp", "0.0.0.0:4022")
	if err != nil {
		// TODO: Do something better
		panic(err)
	}

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Printf("error accepting incoming connection: %s")
			continue
		}

		_, chans, reqs, err := ssh.NewServerConn(clientConn, config)
		if err != nil {
			log.Printf("error during client handshake: %s")
			continue
		}

		go handleClient(chans, reqs)
	}
}

func handleClient(chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		log.Printf("Request for channel of type %q", newChannel.ChannelType())

		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "not supported")
			continue
		}

		channel, channelReqs, err := newChannel.Accept()
		if err != nil {
			log.Printf("error accepting new channel: %s", err)
			break
		}

		go handleClientSession(channel, channelReqs)
	}
}

func handleClientSession(channel ssh.Channel, reqs <-chan *ssh.Request) {
	cmd := exec.Command("/bin/sh")
	cmd.Args[0] = "-sh" // login shell
	cmd.Dir = "/"       // should be user homedir, probably?

	close := func() {
		channel.Close()
		_, err := cmd.Process.Wait()
		if err != nil {
			log.Printf("error waiting for bash to exit: %s", err)
		}
		log.Println("session closed")
	}
	var closeOnce sync.Once

	term, err := pty.Start(cmd)
	if err != nil {
		log.Printf("failed to open pty: %s", err)
		close()
		return
	}

	// Pass data between the channel and the pty
	go func() {
		io.Copy(channel, term)
		closeOnce.Do(close)
	}()
	go func() {
		io.Copy(term, channel)
		closeOnce.Do(close)
	}()

	for req := range reqs {
		log.Printf("session request of type %q", req.Type)
		switch req.Type {
		case "shell":
			if len(req.Payload) > 0 {
				// No explicit command may be specified
				req.Reply(false, nil)
				continue
			}

			req.Reply(true, nil)
			continue

		case "pty-req":
			if len(req.Payload) < 4 {
				log.Println("malformed pty-req payload")
				req.Reply(false, nil)
				continue
			}
			termLen := int(req.Payload[3])
			startIdx := termLen + 4
			if len(req.Payload) < startIdx+8 {
				log.Println("malformed pty-req payload")
				req.Reply(false, nil)
				continue
			}
			size := ParseWinsizeFromSSHMessage(req.Payload[startIdx:])
			log.Printf("Window size is %dx%d", size.Width, size.Height)
			SetPtyWinsize(term, size)
			req.Reply(true, nil)
			continue

		case "window-change":
			if len(req.Payload) < 8 {
				log.Println("malformed window-change payload")
				req.Reply(false, nil)
				continue
			}

			size := ParseWinsizeFromSSHMessage(req.Payload)
			log.Printf("Window size is %dx%d", size.Width, size.Height)
			SetPtyWinsize(term, size)
			req.Reply(true, nil)
			continue

		// TODO: support "env"

		default:
			req.Reply(false, nil)
			continue
		}
	}

}

type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // not used; always zero
	y      uint16 // not used; always zero
}

func ParseWinsizeFromSSHMessage(raw []byte) *Winsize {
	return &Winsize{
		Width:  uint16(binary.BigEndian.Uint32(raw)),
		Height: uint16(binary.BigEndian.Uint32(raw[4:])),
	}
}

func SetPtyWinsize(pty *os.File, size *Winsize) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		pty.Fd(),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(size)),
	)
	if errno != 0 {
		log.Printf("error setting window size: %s", errno.Error())
	}
}
