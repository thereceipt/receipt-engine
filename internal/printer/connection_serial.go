package printer

import (
	"fmt"
	"image"
	"sync"
	
	"github.com/tarm/serial"
)

// SerialConnection represents a serial printer connection
type SerialConnection struct {
	port *serial.Port
	mu   sync.Mutex
}

// ConnectSerial connects to a serial printer
func ConnectSerial(device string, baud int) (*SerialConnection, error) {
	if baud == 0 {
		baud = 9600 // Default baud rate for most thermal printers
	}
	
	config := &serial.Config{
		Name: device,
		Baud: baud,
	}
	
	port, err := serial.OpenPort(config)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port: %w", err)
	}
	
	return &SerialConnection{
		port: port,
	}, nil
}

// Write sends data to the serial printer
func (c *SerialConnection) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	return c.port.Write(data)
}

// Print prints an image to the serial printer
func (c *SerialConnection) Print(img image.Image) error {
	data := EncodeImageToESCPOS(img)
	
	_, err := c.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to serial printer: %w", err)
	}
	
	return nil
}

// Close closes the serial connection
func (c *SerialConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.port != nil {
		return c.port.Close()
	}
	
	return nil
}
