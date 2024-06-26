package ipc

import (
	"bufio"
	"errors"
	"io"
	"log"
	"time"
)

// StartServer - starts the ipc server.
//
// ipcName - is the name of the unix socket or named pipe that will be created, the client needs to use the same name
func StartServer(ipcName string, config *ServerConfig) (*Server, error) {

	err := checkIpcName(ipcName)
	if err != nil {
		return nil, err
	}

	s := &Server{
		name:     ipcName,
		status:   NotConnected,
		received: make(chan *Message),
		toWrite:  make(chan *Message),
	}

	if config == nil {
		s.timeout = 0
		s.maxMsgSize = maxMsgSize
		s.encryption = true
		s.unMask = false

	} else {

		if config.MaxMsgSize < 1024 {
			s.maxMsgSize = maxMsgSize
		} else {
			s.maxMsgSize = config.MaxMsgSize
		}

		if !config.Encryption {
			s.encryption = false
		} else {
			s.encryption = true
		}

		if config.UnmaskPermissions {
			s.unMask = true
		} else {
			s.unMask = false
		}
	}

	err = s.run()

	return s, err
}

func (sc *Server) acceptLoop() {

	for {
		conn, err := sc.listen.Accept()
		if err != nil {
			break
		}

		if sc.status == Listening || sc.status == Disconnected {

			sc.conn = conn

			err2 := sc.handshake()
			if err2 != nil {
				sc.received <- &Message{Err: err2, MsgType: -1}
				sc.status = Error
				sc.listen.Close()
				sc.conn.Close()

			} else {

				go sc.read()
				go sc.write()

				sc.status = Connected
				sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
			}

		}

	}

}

func (sc *Server) read() {

	bLen := make([]byte, 4)

	for {

		res := sc.readData(bLen)
		if !res {
			sc.conn.Close()

			break
		}

		mLen := bytesToInt(bLen)

		msgRecvd := make([]byte, mLen)

		res = sc.readData(msgRecvd)
		if !res {
			sc.conn.Close()

			break
		}

		if sc.encryption {
			msgFinal, err := decrypt(*sc.enc.cipher, msgRecvd)
			if err != nil {
				sc.received <- &Message{Err: err, MsgType: -1}
				continue
			}

			if bytesToInt(msgFinal[:4]) == 0 {
				//  type 0 = control message
			} else {
				sc.received <- &Message{Data: msgFinal[4:], MsgType: bytesToInt(msgFinal[:4])}
			}

		} else {
			if bytesToInt(msgRecvd[:4]) == 0 {
				//  type 0 = control message
			} else {
				sc.received <- &Message{Data: msgRecvd[4:], MsgType: bytesToInt(msgRecvd[:4])}
			}
		}

	}

}

func (sc *Server) readData(buff []byte) bool {

	_, err := io.ReadFull(sc.conn, buff)
	if err != nil {

		if sc.status == Closing {

			sc.status = Closed
			sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
			sc.received <- &Message{Err: errors.New("server has closed the connection"), MsgType: -1}
			return false
		}

		if err == io.EOF {

			sc.status = Disconnected
			sc.received <- &Message{Status: sc.status.String(), MsgType: -1}
			return false
		}

	}

	return true
}

// Read - blocking function, reads each message recieved
// if MsgType is a negative number its an internal message
func (sc *Server) Read() (*Message, error) {

	m, ok := <-sc.received
	if !ok {
		return nil, errors.New("the received channel has been closed")
	}

	if m.Err != nil {
		//close(s.received)
		//close(s.toWrite)
		return nil, m.Err
	}

	return m, nil
}

// Write - writes a message to the ipc connection
// msgType - denotes the type of data being sent. 0 is a reserved type for internal messages and errors.
func (sc *Server) Write(msgType int, message []byte) error {

	if msgType == 0 {
		return errors.New("message type 0 is reserved")
	}

	mlen := len(message)

	if mlen > sc.maxMsgSize {
		return errors.New("message exceeds maximum message length")
	}

	if sc.status == Connected {

		sc.toWrite <- &Message{MsgType: msgType, Data: message}

	} else {
		return errors.New(sc.status.String())
	}

	return nil
}

func (sc *Server) write() {

	for {

		m, ok := <-sc.toWrite

		if !ok {
			break
		}

		toSend := intToBytes(m.MsgType)

		writer := bufio.NewWriter(sc.conn)

		if sc.encryption {
			toSend = append(toSend, m.Data...)
			toSendEnc, err := encrypt(*sc.enc.cipher, toSend)
			if err != nil {
				log.Println("error encrypting data", err)
				continue
			}

			toSend = toSendEnc
		} else {

			toSend = append(toSend, m.Data...)

		}

		writer.Write(intToBytes(len(toSend)))
		writer.Write(toSend)

		err := writer.Flush()
		if err != nil {
			log.Println("error flushing data", err)
			continue
		}

		time.Sleep(2 * time.Millisecond)

	}
}

// getStatus - get the current status of the connection
func (sc *Server) getStatus() Status {

	return sc.status
}

// StatusCode - returns the current connection status
func (sc *Server) StatusCode() Status {
	return sc.status
}

// Status - returns the current connection status as a string
func (sc *Server) Status() string {

	return sc.status.String()
}

// Close - closes the connection
func (sc *Server) Close() {

	sc.status = Closing

	if sc.listen != nil {
		sc.listen.Close()
	}

	if sc.conn != nil {
		sc.conn.Close()
	}
}
