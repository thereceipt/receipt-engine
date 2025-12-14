package printer

import (
	"fmt"
	"image"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// PrinterConnection is a unified interface for all printer types
type PrinterConnection interface {
	Print(img image.Image) error
	Write(data []byte) (int, error)
	Close() error
}

// ConnectionPool manages connections to printers
type ConnectionPool struct {
	connections map[string]PrinterConnection
	mu          sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]PrinterConnection),
	}
}

// Connect establishes a connection to a printer
func (p *ConnectionPool) Connect(printer *Printer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Check if already connected
	if _, exists := p.connections[printer.ID]; exists {
		return nil // Already connected
	}
	
	var conn PrinterConnection
	var err error
	
	switch printer.Type {
	case "usb":
		conn, err = ConnectUSB(printer.VID, printer.PID)
		// If USB connection fails, try serial ports as fallback (common on macOS)
		if err != nil && runtime.GOOS == "darwin" {
			fmt.Printf("⚠️  USB connection failed for %04X:%04X, trying serial port fallback...\n", printer.VID, printer.PID)
			// Try to find a serial port that might be this USB device
			serialPorts := p.findSerialPorts()
			fmt.Printf("🔍 Found %d serial ports to try\n", len(serialPorts))
			for _, port := range serialPorts {
				fmt.Printf("  Trying serial port: %s\n", port)
				serialConn, serialErr := ConnectSerial(port, 9600)
				if serialErr == nil {
					// Serial connection succeeded, use it as fallback
					fmt.Printf("✅ Successfully connected via serial port: %s\n", port)
					conn = serialConn
					err = nil
					break
				} else {
					fmt.Printf("    Failed: %v\n", serialErr)
				}
			}
			if err != nil {
				fmt.Printf("❌ All serial port attempts failed\n")
			}
		}
	case "serial":
		conn, err = ConnectSerial(printer.Device, 9600)
	case "network":
		conn, err = ConnectNetwork(printer.Host, printer.Port)
	default:
		return fmt.Errorf("unsupported printer type: %s", printer.Type)
	}
	
	if err != nil {
		return err
	}
	
	p.connections[printer.ID] = conn
	return nil
}

// findSerialPorts finds available serial ports (helper for USB fallback)
func (p *ConnectionPool) findSerialPorts() []string {
	var ports []string
	
	switch runtime.GOOS {
	case "darwin":
		skipPatterns := []string{"Bluetooth", "Modem", "SPP", "DialIn", "Callout", "KeySerial", "debug-console"}
		cuPorts, _ := filepath.Glob("/dev/cu.*")
		ttyPorts, _ := filepath.Glob("/dev/tty.*")
		allPorts := append(cuPorts, ttyPorts...)
		
		for _, port := range allPorts {
			skip := false
			for _, pattern := range skipPatterns {
				if strings.Contains(port, pattern) {
					skip = true
					break
				}
			}
			if !skip {
				ports = append(ports, port)
			}
		}
	case "linux":
		usbPorts, _ := filepath.Glob("/dev/ttyUSB*")
		acmPorts, _ := filepath.Glob("/dev/ttyACM*")
		ports = append(ports, usbPorts...)
		ports = append(ports, acmPorts...)
	}
	
	return ports
}

// Print sends an image to a printer
func (p *ConnectionPool) Print(printerID string, img image.Image) error {
	p.mu.RLock()
	conn, exists := p.connections[printerID]
	p.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("printer not connected: %s", printerID)
	}
	
	return conn.Print(img)
}

// Disconnect closes a printer connection
func (p *ConnectionPool) Disconnect(printerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	conn, exists := p.connections[printerID]
	if !exists {
		return nil // Already disconnected
	}
	
	err := conn.Close()
	delete(p.connections, printerID)
	
	return err
}

// DisconnectAll closes all connections
func (p *ConnectionPool) DisconnectAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for id, conn := range p.connections {
		conn.Close()
		delete(p.connections, id)
	}
}

// IsConnected checks if a printer is connected
func (p *ConnectionPool) IsConnected(printerID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	_, exists := p.connections[printerID]
	return exists
}
