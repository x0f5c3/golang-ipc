package ipc

import (
	"errors"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

// Server function
// Create the named pipe (if it doesn't already exist) and start listening for a client to connect.
// when a client connects and connection is accepted the read function is called on a go routine.
func (sc *Server) run() error {

	var pipeBase = `\\.\pipe\`

	var config *winio.PipeConfig

	if sc.unMask {
		config = &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;AU)", InputBufferSize: int32(sc.maxMsgSize), OutputBufferSize: int32(sc.maxMsgSize)}
	}

	listen, err := winio.ListenPipe(pipeBase+sc.name, config)
	if err != nil {

		return err
	}

	sc.listen = listen

	sc.status = Listening

	go sc.acceptLoop()

	return nil

}

// Client function
// dial - attempts to connect to a named pipe created by the server
func (c *Client) dial() error {

	var pipeBase = `\\.\pipe\`

	startTime := time.Now()

	for {
		if c.timeout != 0 {
			if time.Since(startTime).Seconds() > c.timeout {
				c.status = Closed
				return errors.New("timed out trying to connect")
			}
		}
		pn, err := winio.DialPipe(pipeBase+c.Name, nil)
		if err != nil {

			if strings.Contains(err.Error(), "the system cannot find the file specified.") == true {

			} else {
				return err
			}

		} else {

			c.conn = pn

			err = c.handshake()
			if err != nil {
				return err
			}
			return nil
		}

		time.Sleep(c.retryTimer * time.Second)

	}
}
