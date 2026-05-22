package wsconn

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Conn wraps a *websocket.Conn to implement net.Conn for yamux.
type Conn struct {
	ws     *websocket.Conn
	reader io.Reader
	mu     sync.Mutex
}

func New(ws *websocket.Conn) *Conn {
	return &Conn{ws: ws}
}

func (c *Conn) Read(p []byte) (int, error) {
	if c.reader == nil {
		_, r, err := c.ws.NextReader()
		if err != nil {
			return 0, err
		}
		c.reader = r
	}
	n, err := c.reader.Read(p)
	if err == io.EOF {
		c.reader = nil
		if n > 0 {
			return n, nil
		}
		return c.Read(p)
	}
	return n, err
}

func (c *Conn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.ws.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *Conn) Close() error                       { return c.ws.Close() }
func (c *Conn) LocalAddr() net.Addr                { return c.ws.LocalAddr() }
func (c *Conn) RemoteAddr() net.Addr               { return c.ws.RemoteAddr() }
func (c *Conn) SetDeadline(t time.Time) error      { return nil }
func (c *Conn) SetReadDeadline(t time.Time) error  { return nil }
func (c *Conn) SetWriteDeadline(t time.Time) error { return nil }
