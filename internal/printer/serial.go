package printer

import (
	"fmt"
	"path/filepath"
	"runtime"
	
	"github.com/tarm/serial"
)

// detectSerialPorts scans for serial ports that might be printers
func (m *Manager) detectSerial() ([]*Printer, error) {
	var printers []*Printer
	var ports []string
	
	switch runtime.GOOS {
	case "darwin":
		// macOS: Scan /dev/cu.* and /dev/tty.*
		ports = scanMacOSPorts()
	case "linux":
		// Linux: Scan /dev/ttyUSB*, /dev/ttyACM*, etc.
		ports = scanLinuxPorts()
	case "windows":
		// Windows: Scan COM1-COM256
		ports = scanWindowsPorts()
	default:
		return printers, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
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
		info := registry.PrinterInfo{
			Type:        "serial",
			Device:      portPath,
			Description: fmt.Sprintf("Serial: %s", filepath.Base(portPath)),
		}
		
		id := m.registry.GetPrinterID(info)
		
		printer := &Printer{
			ID:          id,
			Type:        "serial",
			Description: info.Description,
			Device:      portPath,
			Name:        m.registry.GetPrinterName(id),
		}
		
		printers = append(printers, printer)
	}
	
	return printers, nil
}

func scanMacOSPorts() []string {
	var ports []string
	
	// Common patterns for macOS
	patterns := []string{
		"/dev/cu.*",
		"/dev/tty.*",
	}
	
	// Skip Bluetooth and other non-printer devices
	skipPatterns := []string{
		"Bluetooth",
		"debug-console",
		"KeySerial",
	}
	
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			skip := false
			for _, skipPattern := range skipPatterns {
				if contains(match, skipPattern) {
					skip = true
					break
				}
			}
			if !skip {
				ports = append(ports, match)
			}
		}
	}
	
	return ports
}

func scanLinuxPorts() []string {
	var ports []string
	
	patterns := []string{
		"/dev/ttyUSB*",
		"/dev/ttyACM*",
		"/dev/ttyS*",
	}
	
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		ports = append(ports, matches...)
	}
	
	return ports
}

func scanWindowsPorts() []string {
	var ports []string
	
	// Windows COM ports
	for i := 1; i <= 256; i++ {
		ports = append(ports, fmt.Sprintf("COM%d", i))
	}
	
	return ports
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr)*2 && contains(s[1:len(s)-1], substr)
}
