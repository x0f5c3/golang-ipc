//go:build linux || darwin
// +build linux darwin

package ipc

import (
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// Server create a unix socket and start listening connections - for unix and linux
func (sc *Server) run() error {

	base := "/tmp/"
	sock := ".sock"

	if err := os.RemoveAll(base + sc.name + sock); err != nil {
		return err
	}

	var oldUmask int
	if sc.unMask {
		oldUmask = syscall.Umask(0)
	}

	listen, err := net.Listen("unix", base+sc.name+sock)

	if sc.unMask {
		syscall.Umask(oldUmask)
	}

	if err != nil {
		return err
	}

	sc.listen = listen

	go sc.acceptLoop()

	sc.status = Listening

	return nil

}

// Client connect to the unix socket created by the server -  for unix and linux
func (c *Client) dial() error {

	base := "/tmp/"
	sock := ".sock"

	startTime := time.Now()

	for {

		if c.timeout != 0 {

			if time.Since(startTime).Seconds() > c.timeout {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}

		conn, err := net.Dial("unix", base+c.Name+sock)
		if err != nil {

			if strings.Contains(err.Error(), "connect: no such file or directory") {

			} else if strings.Contains(err.Error(), "connect: connection refused") {

			} else {
				c.received <- &Message{Err: err, MsgType: -1}
			}

		} else {

			c.conn = conn

			err = c.handshake()
			if err != nil {
				return err
			}

			return nil
		}

		time.Sleep(c.retryTimer * time.Second)

	}

}
