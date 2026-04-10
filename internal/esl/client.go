// Package esl is a thin wrapper around fiorix/go-eventsocket that exposes
// only the operations fs-inspect needs: connect, run an API command, read
// the body of the response.
package esl

import (
	"fmt"

	eventsocket "github.com/fiorix/go-eventsocket/eventsocket"
)

// Client is a single-shot ESL connection to one FreeSWITCH instance.
type Client struct {
	conn *eventsocket.Connection
	addr string
}

// Dial opens an inbound ESL connection to addr (e.g. "127.0.0.1:8021")
// and authenticates with password.
func Dial(addr, password string) (*Client, error) {
	conn, err := eventsocket.Dial(addr, password)
	if err != nil {
		return nil, fmt.Errorf("esl dial %s: %w", addr, err)
	}
	return &Client{conn: conn, addr: addr}, nil
}

// API runs a FreeSWITCH API command (e.g. "show channels as json") and
// returns the raw response body.
func (c *Client) API(cmd string) (string, error) {
	ev, err := c.conn.Send("api " + cmd)
	if err != nil {
		return "", fmt.Errorf("esl api %q on %s: %w", cmd, c.addr, err)
	}
	return ev.Body, nil
}

// Close releases the underlying TCP connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}
