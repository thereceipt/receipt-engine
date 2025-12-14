package printer

import (
	"fmt"
	"image"
	"net"
	"sync"
	"time"
)

// NetworkConnection represents a network printer connection
type NetworkConnection struct {
	conn net.Conn
	mu   sync.Mutex
}

// ConnectNetwork connects to a network printer
func ConnectNetwork(host string, port int) (*NetworkConnection, error) {
	address := fmt.Sprintf("%s:%d", host, port)
	
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to network printer: %w", err)
	}
	
	return &NetworkConnection{
		conn: conn,
	}, nil
}

// Write sends data to the network printer
func (c *NetworkConnection) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	return c.conn.Write(data)
}

// Print prints an image to the network printer
func (c *NetworkConnection) Print(img image.Image) error {
	data := EncodeImageToESCPOS(img)
	
	_, err := c.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to network printer: %w", err)
	}
	
	return nil
}

// Close closes the network connection
func (c *NetworkConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		return c.conn.Close()
	}
	
	return nil
}
