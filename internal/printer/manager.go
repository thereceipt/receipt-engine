// Package printer handles printer detection, connection, and communication
package printer

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/gousb"
	"github.com/tarm/serial"
	"github.com/thereceipt/receipt-engine/internal/registry"
)

// Manager handles printer detection and management
type Manager struct {
	registry *registry.Registry
	printers map[string]*Printer
	mu       sync.RWMutex

	// Event callbacks
	onPrinterAdded   func(*Printer)
	onPrinterRemoved func(string)
}

// Printer represents a detected printer
type Printer struct {
	ID          string
	Type        string // usb, serial, network
	Description string
	Device      string
	VID         uint16
	PID         uint16
	Host        string
	Port        int
	Name        string // Custom user-set name
}

// NewManager creates a new printer manager
func NewManager(registryPath string) (*Manager, error) {
	reg, err := registry.New(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	return &Manager{
		registry: reg,
		printers: make(map[string]*Printer),
	}, nil
}

// DetectPrinters scans for all available printers
func (m *Manager) DetectPrinters() ([]*Printer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var printers []*Printer

	// Detect USB printers
	usbPrinters, err := m.detectUSB()
	if err != nil {
		fmt.Printf("Warning: USB detection failed: %v\n", err)
	} else {
		printers = append(printers, usbPrinters...)
	}

	// Detect Serial printers
	serialPrinters, err := m.detectSerial()
	if err != nil {
		fmt.Printf("Warning: Serial detection failed: %v\n", err)
	} else {
		printers = append(printers, serialPrinters...)
	}

	// Update internal printer map
	m.printers = make(map[string]*Printer)
	for _, p := range printers {
		m.printers[p.ID] = p
	}

	return printers, nil
}

// GetPrinter returns a printer by ID
func (m *Manager) GetPrinter(id string) *Printer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.printers[id]
}

// GetAllPrinters returns all detected printers
func (m *Manager) GetAllPrinters() []*Printer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Printer, 0, len(m.printers))
	for _, p := range m.printers {
		result = append(result, p)
	}
	return result
}

// SetPrinterName sets a custom name for a printer
func (m *Manager) SetPrinterName(id string, name string) bool {
	success := m.registry.SetPrinterName(id, name)

	if success {
		m.mu.Lock()
		if printer, exists := m.printers[id]; exists {
			printer.Name = name
		}
		m.mu.Unlock()
	}

	return success
}

// AddNetworkPrinter manually adds a network printer
func (m *Manager) AddNetworkPrinter(host string, port int, description string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	info := registry.PrinterInfo{
		Type:        "network",
		Host:        host,
		Port:        port,
		Description: description,
	}

	id := m.registry.GetPrinterID(info)

	printer := &Printer{
		ID:          id,
		Type:        "network",
		Description: description,
		Host:        host,
		Port:        port,
		Name:        m.registry.GetPrinterName(id),
	}

	m.printers[id] = printer

	return id
}

// OnPrinterAdded sets a callback for when a printer is added
func (m *Manager) OnPrinterAdded(callback func(*Printer)) {
	m.onPrinterAdded = callback
}

// OnPrinterRemoved sets a callback for when a printer is removed
func (m *Manager) OnPrinterRemoved(callback func(string)) {
	m.onPrinterRemoved = callback
}

// detectUSB detects USB printers using libusb
func (m *Manager) detectUSB() ([]*Printer, error) {
	// Initialize USB context
	ctx := gousb.NewContext()
	defer ctx.Close()

	// Suppress libusb interrupted errors (code -10) - these are harmless
	// They occur during normal USB enumeration and don't affect functionality

	var printers []*Printer

	// Find ALL USB devices (don't filter by VID/PID)
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// Accept all devices - we'll check class later
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}

	for _, dev := range devices {
		// Get device descriptor
		desc := dev.Desc

		// Check if device is a printer (class 7 = printer)
		isPrinter := false

		// Check device class
		if desc.Class == gousb.ClassPrinter {
			isPrinter = true
		}

		// Also check interface classes
		if !isPrinter {
			for _, cfg := range desc.Configs {
				for _, iface := range cfg.Interfaces {
					for _, alt := range iface.AltSettings {
						if alt.Class == gousb.ClassPrinter {
							isPrinter = true
							break
						}
					}
					if isPrinter {
						break
					}
				}
				if isPrinter {
					break
				}
			}
		}

		// If not a printer class device, skip it
		if !isPrinter {
			dev.Close()
			continue
		}

		// Get device strings
		manufacturer, _ := dev.Manufacturer()
		product, _ := dev.Product()

		description := fmt.Sprintf("USB: %04X:%04X", desc.Vendor, desc.Product)
		if manufacturer != "" || product != "" {
			description = fmt.Sprintf("USB: %s %s (%04X:%04X)",
				manufacturer, product, desc.Vendor, desc.Product)
		}

		// Create printer info
		info := registry.PrinterInfo{
			Type:        "usb",
			VID:         uint16(desc.Vendor),
			PID:         uint16(desc.Product),
			Description: description,
		}

		id := m.registry.GetPrinterID(info)

		printer := &Printer{
			ID:          id,
			Type:        "usb",
			Description: description,
			VID:         uint16(desc.Vendor),
			PID:         uint16(desc.Product),
			Name:        m.registry.GetPrinterName(id),
		}

		printers = append(printers, printer)
		dev.Close()
	}

	return printers, nil
}

// detectSerial detects serial printers
func (m *Manager) detectSerial() ([]*Printer, error) {
	var printers []*Printer
	var ports []string

	switch runtime.GOOS {
	case "darwin":
		// macOS: Scan /dev/cu.* and /dev/tty.*
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
		// Linux: Scan /dev/ttyUSB*, /dev/ttyACM*, etc.
		usbPorts, _ := filepath.Glob("/dev/ttyUSB*")
		acmPorts, _ := filepath.Glob("/dev/ttyACM*")
		sPorts, _ := filepath.Glob("/dev/ttyS*")
		ports = append(ports, usbPorts...)
		ports = append(ports, acmPorts...)
		ports = append(ports, sPorts...)

	case "windows":
		// Windows: Test COM1-COM256
		for i := 1; i <= 256; i++ {
			ports = append(ports, fmt.Sprintf("COM%d", i))
		}
	}

	// Test each port
	for _, portPath := range ports {
		// Try to open the port briefly to verify it exists
		config := &serial.Config{
			Name: portPath,
			Baud: 9600,
		}

		port, err := serial.OpenPort(config)
		if err != nil {
			// Skip ports that can't be opened
			continue
		}
		port.Close()

		// Create printer info
		description := fmt.Sprintf("Serial: %s", filepath.Base(portPath))

		info := registry.PrinterInfo{
			Type:        "serial",
			Device:      portPath,
			Description: description,
		}

		id := m.registry.GetPrinterID(info)

		printer := &Printer{
			ID:          id,
			Type:        "serial",
			Description: description,
			Device:      portPath,
			Name:        m.registry.GetPrinterName(id),
		}

		printers = append(printers, printer)
	}

	return printers, nil
}
