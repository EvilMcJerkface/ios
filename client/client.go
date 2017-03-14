package main

import (
	"bufio"
	"errors"
	"github.com/golang/glog"
	"github.com/heidi-ann/ios/msgs"
	"io"
	"net"
	"time"
)

type client struct {
	id        int
	requestID int //TODO: write this value to disk
	stats     *statsFile
	servers   []string
	conn      net.Conn
	rd        *bufio.Reader
	timeout   time.Duration
	master    int
}

func connect(addrs []string, tries int, hint int) (net.Conn, int, error) {
	var conn net.Conn
	var err error

	// reset invalid hint
	if len(addrs) >= hint {
		hint = 0
	}

	// first, try on to connect to the most likely leader
	glog.V(1).Info("Trying to connect to ", addrs[hint])
	conn, err = net.Dial("tcp", addrs[hint])
	// if successful
	if err == nil {
		glog.V(1).Infof("Connect established to %s", addrs[hint])
		return conn, hint, err
	}
	//if unsuccessful
	glog.Warning(err)

	// if fails, try everyone else
	for i := range addrs {
		for t := tries; t > 0; t-- {
			glog.V(1).Info("Trying to connect to ", addrs[i])
			conn, err = net.Dial("tcp", addrs[i])

			// if successful
			if err == nil {
				glog.V(1).Infof("Connect established to %s", addrs[i])
				return conn, i, err
			}

			//if unsuccessful
			glog.Warning(err)
			time.Sleep(100 * time.Millisecond)
		}
	}

	// calc most likely next leader
	hint += 1
	if len(addrs) == hint {
		hint = 0
	}
	return conn, hint, err
}

// send bytes and wait for reply, return bytes returned if succussful or error otherwise
func dispatcher(b []byte, conn net.Conn, r *bufio.Reader, timeout time.Duration) ([]byte, error) {
	// setup channels for timeout implementation
	errCh := make(chan error, 1)
	replyCh := make(chan []byte, 1)

	go func() {
		// send request
		_, err := conn.Write(b)
		_, err = conn.Write([]byte("\n"))
		if err != nil && err != io.EOF {
			glog.Warning(err)
			errCh <- err
		}
		glog.V(1).Info("Sent")
		// read response
		reply, err := r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			glog.Warning(err)
			errCh <- err
		}
		// success, return reply
		replyCh <- reply
	}()

	//handling outcomes
	select {
	case reply := <-replyCh:
		return reply, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(timeout):
		return nil, errors.New("Timeout")
	}
}

func StartClient(id int, statFile string, addrs []string, timeout time.Duration) *client {
	glog.Info("Starting up client ", id)

	// set up stats collection
	stats := OpenStatsFile(statFile)

	// connecting to server
	conn, master, err := connect(addrs, 10, 0)
	if err != nil {
		glog.Fatal(err)
	}
	rd := bufio.NewReader(conn)

	glog.Info("Client is ready to start processing incoming requests")
	return &client{id, 1, stats, addrs, conn, rd, timeout, master}
}

func (c *client) SubmitRequest(text string) (bool, string) {
	glog.V(1).Info("Request ", c.requestID, " is: ", text)

	// prepare request
	req := msgs.ClientRequest{c.id, c.requestID, false, text}
	b, err := msgs.Marshal(req)
	if err != nil {
		glog.Fatal(err)
	}
	glog.V(1).Info(string(b))

	c.stats.StartRequest(c.requestID)
	tries := 0
	var reply *msgs.ClientResponse

	// dispatch request until successful
	for {
		tries++
		if tries > len(c.servers) {
			req.ForceViewChange = true
			b, err = msgs.Marshal(req)
			if err != nil {
				glog.Fatal(err)
			}
		}

		replyBytes, err := dispatcher(b, c.conn, c.rd, c.timeout)
		if err == nil {
			//handle reply
			reply = new(msgs.ClientResponse)
			err = msgs.Unmarshal(replyBytes, reply)

			if err == nil && !reply.Success {
				err = errors.New("request marked by server as unsuccessful")
			}
			if err == nil && reply.Success {
				glog.V(1).Info("request was Successful", reply)
				break
			}
		}
		glog.Warning("Request ", c.requestID, " failed due to: ", err)

		// try to establish a new connection
		for {
			if c.conn != nil {
				err = c.conn.Close()
				if err != nil {
					glog.Warning(err)
				}
			}
			c.conn, c.master, err = connect(c.servers, 10, c.master+1)
			if err == nil {
				break
			}
			glog.Warning("Serious connectivity issues")
			time.Sleep(time.Second)
		}

		c.rd = bufio.NewReader(c.conn)

	}

	//check reply is not nil
	if *reply == (msgs.ClientResponse{}) {
		glog.Fatal("Response is nil")
	}

	//check reply is as expected
	if reply.ClientID != *id {
		glog.Fatal("Response received has wrong ClientID: expected ",
			*id, " ,received ", reply.ClientID)
	}
	if reply.RequestID != c.requestID {
		glog.Fatal("Response received has wrong RequestID: expected ",
			c.requestID, " ,received ", reply.RequestID)
	}
	if !reply.Success {
		glog.Fatal("Response marked as unsuccessful but not retried")
	}

	// write to latency to log
	c.stats.StopRequest(tries)
	c.requestID++
	return true, reply.Response
}

func (c *client) StopClient() {
	glog.V(1).Info("Shutting down client ", *id)
	// close stats file
	c.stats.CloseStatsFile()
	// close connection
	if c.conn != nil {
		c.conn.Close()
	}
}
