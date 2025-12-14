// Package printer handles printer detection, connection, and communication
package printer

import (
	"fmt"
	"sync"

	"github.com/yourusername/receipt-engine/internal/registry"
)

// Manager handles printer detection and management
type Manager struct {
	registry  *registry.Registry
	printers  map[string]*Printer
	mu        sync.RWMutex
	
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

// detectUSB detects USB printers
func (m *Manager) detectUSB() ([]*Printer, error) {
	// TODO: Implement USB detection using github.com/google/gousb
	// This requires libusb to be installed
	// For now, return empty list
	return []*Printer{}, nil
}

// detectSerial detects serial printers
func (m *Manager) detectSerial() ([]*Printer, error) {
	// TODO: Implement serial detection
	// Scan /dev/cu.* and /dev/tty.* on macOS
	// Scan COM ports on Windows
	// For now, return empty list
	return []*Printer{}, nil
}
