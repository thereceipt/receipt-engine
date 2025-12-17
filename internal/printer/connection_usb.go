package printer

import (
	"fmt"
	"image"
	"sync"
	"time"
	
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
// Returns error if USB support is not available (libusb not installed)
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
	
	// Try without SetAutoDetach first (some devices work without it)
	// First, try the simple approach: DefaultInterface (interface 0, alt setting 0)
	// This works for most printers
	iface, done, err := dev.DefaultInterface()
	if err != nil {
		// If that failed, try with SetAutoDetach
		dev.SetAutoDetach(true)
		iface, done, err = dev.DefaultInterface()
	}
	if err == nil {
		// Find OUT endpoint
		var outEndpoint *gousb.OutEndpoint
		for _, epDesc := range iface.Setting.Endpoints {
			if epDesc.Direction == gousb.EndpointDirectionOut {
				ep, err := iface.OutEndpoint(epDesc.Number)
				if err == nil {
					outEndpoint = ep
					break
				}
			}
		}
		
		if outEndpoint != nil {
			// Success with DefaultInterface!
			_ = done // Will be called on close
			conn := &USBConnection{
				device:   dev,
				iface:    iface,
				endpoint: outEndpoint,
			}
			return conn, nil
		}
		
		// No OUT endpoint found, close and try enumeration
		iface.Close()
	}
	
	// DefaultInterface failed, try enumerating all configurations and interfaces
	desc := dev.Desc
	var lastErr error
	
	// First, try to get the active configuration (device might already be configured)
	activeCfg, activeCfgErr := dev.ActiveConfigNum()
	if activeCfgErr == nil && activeCfg > 0 {
		// Device has an active config, try using it
		cfg, err := dev.Config(activeCfg)
		if err == nil {
			// Find the config descriptor
			cfgDesc, exists := desc.Configs[activeCfg]
			if exists {
				for _, ifaceDesc := range cfgDesc.Interfaces {
					ifaceNum := ifaceDesc.Number
					iface, err := cfg.Interface(ifaceNum, 0)
					if err == nil {
						// Find OUT endpoint
						var outEndpoint *gousb.OutEndpoint
						for _, epDesc := range iface.Setting.Endpoints {
							if epDesc.Direction == gousb.EndpointDirectionOut {
								ep, err := iface.OutEndpoint(epDesc.Number)
								if err == nil {
									outEndpoint = ep
									break
								}
							}
						}
						if outEndpoint != nil {
							conn := &USBConnection{
								device:   dev,
								iface:    iface,
								endpoint: outEndpoint,
							}
							return conn, nil
						}
						iface.Close()
					}
				}
				cfg.Close()
			}
		}
	}
	
	// If active config didn't work, try setting each configuration
	for _, cfgDesc := range desc.Configs {
		// Try to set this configuration
		cfg, err := dev.Config(cfgDesc.Number)
		if err != nil {
			lastErr = fmt.Errorf("failed to set config %d: %w", cfgDesc.Number, err)
			continue
		}
		
		// Try each interface in this configuration
		for _, ifaceDesc := range cfgDesc.Interfaces {
			ifaceNum := ifaceDesc.Number
			
			// Try to claim the interface
			// SetAutoDetach should handle kernel driver detachment automatically
			iface, err := cfg.Interface(ifaceNum, 0)
			if err != nil {
				lastErr = fmt.Errorf("failed to claim interface %d: %w", ifaceNum, err)
				// Try with a brief delay in case device needs time
				time.Sleep(100 * time.Millisecond)
				iface, err = cfg.Interface(ifaceNum, 0)
				if err != nil {
					lastErr = fmt.Errorf("failed to claim interface %d (retry): %w", ifaceNum, err)
					continue
				}
			}
			
			// Find OUT endpoint in this interface
			var outEndpoint *gousb.OutEndpoint
			for _, epDesc := range iface.Setting.Endpoints {
				if epDesc.Direction == gousb.EndpointDirectionOut {
					ep, err := iface.OutEndpoint(epDesc.Number)
					if err == nil {
						outEndpoint = ep
						break
					}
				}
			}
			
			if outEndpoint != nil {
				// Success! Return the connection
				conn := &USBConnection{
					device:   dev,
					iface:    iface,
					endpoint: outEndpoint,
				}
				return conn, nil
			}
			
			// No OUT endpoint in this interface, close it and try next
			iface.Close()
		}
		
		// Close config if we didn't find a working interface
		cfg.Close()
	}
	
	// If we get here, nothing worked
	dev.Close()
	ctx.Close()
	
	if lastErr != nil {
		return nil, fmt.Errorf("failed to connect to USB printer: %w", lastErr)
	}
	return nil, fmt.Errorf("no suitable interface/endpoint found for USB printer %04X:%04X", vid, pid)
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
