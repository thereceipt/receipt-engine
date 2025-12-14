package printer

import (
	"fmt"
	"image"
	"sync"
	
	"github.com/google/gousb"
)

// USBConnection represents a USB printer connection
type USBConnection struct {
	device   *gousb.Device
	iface    *gousb.Interface
	endpoint *gousb.OutEndpoint
	mu       sync.Mutex
}

// ConnectUSB connects to a USB printer
func ConnectUSB(vid, pid uint16) (*USBConnection, error) {
	ctx := gousb.NewContext()
	
	// Find device
	dev, err := ctx.OpenDeviceWithVIDPID(gousb.ID(vid), gousb.ID(pid))
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("failed to open USB device: %w", err)
	}
	
	if dev == nil {
		ctx.Close()
		return nil, fmt.Errorf("device not found: %04X:%04X", vid, pid)
	}
	
	// Set auto-detach kernel driver
	dev.SetAutoDetach(true)
	
	// Claim interface 0 (most printers use interface 0)
	iface, done, err := dev.DefaultInterface()
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("failed to claim interface: %w", err)
	}
	
	// Find OUT endpoint
	var outEndpoint *gousb.OutEndpoint
	for _, desc := range iface.Setting.Endpoints {
		if desc.Direction == gousb.EndpointDirectionOut {
			ep, err := iface.OutEndpoint(desc.Number)
			if err == nil {
				outEndpoint = ep
				break
			}
		}
	}
	
	if outEndpoint == nil {
		done()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("no OUT endpoint found")
	}
	
	conn := &USBConnection{
		device:   dev,
		iface:    iface,
		endpoint: outEndpoint,
	}
	
	return conn, nil
}

// Write sends data to the USB printer
func (c *USBConnection) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	return c.endpoint.Write(data)
}

// Print prints an image to the USB printer
func (c *USBConnection) Print(img image.Image) error {
	data := EncodeImageToESCPOS(img)
	
	_, err := c.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to USB printer: %w", err)
	}
	
	return nil
}

// Close closes the USB connection
func (c *USBConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.iface != nil {
		c.iface.Close()
	}
	
	if c.device != nil {
		c.device.Close()
	}
	
	return nil
}
