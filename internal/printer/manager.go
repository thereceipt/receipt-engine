// Package printer handles printer detection, connection, and communication
package printer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/gousb"
	"github.com/thereceipt/receipt-engine/internal/registry"
)

// Manager handles printer detection and management
type Manager struct {
	registry           *registry.Registry
	printers           map[string]*Printer
	mu                 sync.RWMutex
	networkScanStarted bool
	networkScanMu      sync.Mutex

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
		registry:           reg,
		printers:           make(map[string]*Printer),
		networkScanStarted: false,
	}, nil
}

// DetectPrinters scans for all available printers
func (m *Manager) DetectPrinters() ([]*Printer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var printers []*Printer

	// Detect USB printers (gracefully degrades if libusb not available)
	usbPrinters, err := m.detectUSB()
	if err != nil {
		// USB detection failed - likely libusb not available, skip silently
		// This is expected on systems without libusb installed
	} else {
		printers = append(printers, usbPrinters...)
	}

	// Detect Serial printers (includes USB printers that appear as serial devices)
	serialPrinters, err := m.detectSerial()
	if err != nil {
		// Silently skip - errors are expected and handled gracefully
	} else {
		printers = append(printers, serialPrinters...)
	}

	// Detect Network printers
	networkPrinters, err := m.detectNetwork()
	if err != nil {
		// Silently skip - errors are expected and handled gracefully
	} else {
		printers = append(printers, networkPrinters...)
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

	// Trigger callback if set
	if m.onPrinterAdded != nil {
		m.onPrinterAdded(printer)
	}

	// Log will be handled by caller via callbacks

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
// Gracefully returns empty list if libusb is not available (no error)
func (m *Manager) detectUSB() ([]*Printer, error) {
	// Try to initialize USB context - if this fails, libusb is not available
	// We use a recover to catch any panics from gousb when libusb is missing
	defer func() {
		if r := recover(); r != nil {
			// gousb can panic if libusb is not available - this is expected
		}
	}()

	ctx := gousb.NewContext()
	if ctx == nil {
		return []*Printer{}, nil // USB not available, return empty list
	}
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
		// USB enumeration failed - likely libusb issue, return empty list
		return []*Printer{}, nil
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

// detectSerial detects serial printers (including USB printers that appear as serial devices)
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
		// Windows: Limit to COM1-COM32 (much faster than testing 256 ports)
		for i := 1; i <= 32; i++ {
			ports = append(ports, fmt.Sprintf("COM%d", i))
		}
	}

	// Add all found ports without testing (much faster)
	// On Unix, filepath.Glob already verified the files exist
	// On Windows, we'll just list them - user can test manually if needed
	for _, portPath := range ports {
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

// detectNetwork detects network printers using system services and network scanning
func (m *Manager) detectNetwork() ([]*Printer, error) {
	var printers []*Printer

	// First, try to detect printers from system printer services (CUPS on macOS/Linux)
	// This is fast and non-blocking
	systemPrinters, err := m.detectSystemPrinters()
	if err == nil {
		printers = append(printers, systemPrinters...)
	}

	// Start background network scanning ONLY ONCE (non-blocking)
	// This will discover printers and add them asynchronously
	m.networkScanMu.Lock()
	if !m.networkScanStarted {
		m.networkScanStarted = true
		m.networkScanMu.Unlock()
		go m.scanNetworkInBackground()
	} else {
		m.networkScanMu.Unlock()
	}

	return printers, nil
}

// scanNetworkInBackground scans the network for printers in the background
// and adds them to the manager as they're discovered
func (m *Manager) scanNetworkInBackground() {
	// Background scanning - no console output to avoid messing up TUI

	// Get local network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		// Silently fail - network scanning is best effort
		return
	}

	// Common ports for receipt printers
	ports := []int{9100, 515} // Raw printing (9100) and LPR (515)

	// Scan each interface's network
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			// Skip IPv6 for now
			if ip.To4() == nil {
				continue
			}

			// Get network base and mask
			mask := ipNet.Mask
			baseIP := ip.Mask(mask)

			// Scan a limited range (first 50 IPs) to avoid long delays
			ip4 := baseIP.To4()
			if ip4 == nil {
				continue
			}

			// Scan first 100 IPs in the subnet (increased from 50)
			for i := 1; i <= 100 && i < 256; i++ {
				testIP := make(net.IP, 4)
				copy(testIP, ip4)
				testIP[3] = byte(int(ip4[3]) + i)

				// Skip broadcast and network addresses
				if testIP.Equal(ipNet.IP) {
					continue
				}

				// Test each port with a shorter timeout
				for _, port := range ports {
					address := net.JoinHostPort(testIP.String(), fmt.Sprint(port))
					conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
					if err == nil {
						conn.Close()
						// Found a printer on this port - will be added via callback
						description := fmt.Sprintf("Network: %s:%d", testIP.String(), port)
						info := registry.PrinterInfo{
							Type:        "network",
							Host:        testIP.String(),
							Port:        port,
							Description: description,
						}
						id := m.registry.GetPrinterID(info)

						// Check if we already have this printer
						m.mu.RLock()
						_, exists := m.printers[id]
						m.mu.RUnlock()

						if !exists {
							printer := &Printer{
								ID:          id,
								Type:        "network",
								Description: description,
								Host:        testIP.String(),
								Port:        port,
								Name:        m.registry.GetPrinterName(id),
							}

							// Add to manager
							m.mu.Lock()
							m.printers[id] = printer
							m.mu.Unlock()

							// Trigger callback - will log to TUI

							// Trigger callback if set
							if m.onPrinterAdded != nil {
								m.onPrinterAdded(printer)
							}
						}
						// Found a printer on this IP, no need to check other ports
						break
					}
				}
			}
		}
	}
}

// detectSystemPrinters detects network printers from system printer services
func (m *Manager) detectSystemPrinters() ([]*Printer, error) {
	var printers []*Printer

	switch runtime.GOOS {
	case "darwin", "linux":
		// Use lpstat to query CUPS for network printers
		// Check if lpstat is available first
		if _, err := exec.LookPath("lpstat"); err != nil {
			// lpstat not available, skip system printer detection
			return printers, nil
		}

		// Set a timeout for the command to avoid hanging
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "lpstat", "-v")
		output, err := cmd.Output()
		if err != nil {
			// If command fails or times out, just return empty list (non-fatal)
			return printers, nil
		}

		// Parse lpstat output to find network printers
		// Format: device for PRINTER_NAME: network/ipp://HOST:PORT/ipp/print
		// or: device for PRINTER_NAME: socket://HOST:PORT
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		deviceRe := regexp.MustCompile(`device for ([^:]+):\s*(.+)`)
		networkRe := regexp.MustCompile(`(?:socket|ipp|http)://([^:/]+):?(\d+)?`)

		for scanner.Scan() {
			line := scanner.Text()
			matches := deviceRe.FindStringSubmatch(line)
			if len(matches) < 3 {
				continue
			}

			printerName := strings.TrimSpace(matches[1])
			deviceURI := strings.TrimSpace(matches[2])

			// Check if it's a network printer
			networkMatches := networkRe.FindStringSubmatch(deviceURI)
			if len(networkMatches) < 2 {
				continue
			}

			host := networkMatches[1]
			port := 9100 // Default raw printing port
			if len(networkMatches) > 2 && networkMatches[2] != "" {
				// Try to parse port from URI
				fmt.Sscanf(networkMatches[2], "%d", &port)
			}

			// For IPP/HTTP printers, try port 9100 as well (raw printing)
			// Many network printers support both IPP and raw TCP
			if strings.Contains(deviceURI, "ipp://") || strings.Contains(deviceURI, "http://") {
				// Also add raw TCP port 9100 version
				description := fmt.Sprintf("Network: %s (%s)", printerName, host)
				info := registry.PrinterInfo{
					Type:        "network",
					Host:        host,
					Port:        9100,
					Description: description,
				}
				id := m.registry.GetPrinterID(info)
				printer := &Printer{
					ID:          id,
					Type:        "network",
					Description: description,
					Host:        host,
					Port:        9100,
					Name:        m.registry.GetPrinterName(id),
				}
				printers = append(printers, printer)
			} else if strings.Contains(deviceURI, "socket://") {
				// Raw socket printer
				description := fmt.Sprintf("Network: %s (%s:%d)", printerName, host, port)
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
				printers = append(printers, printer)
			}
		}

	case "windows":
		// On Windows, we could use wmic or PowerShell to query printers
		// For now, we'll rely on network scanning
	}

	return printers, nil
}
