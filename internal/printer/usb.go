package printer

import (
	"fmt"
	
	"github.com/google/gousb"
	"github.com/yourusername/receipt-engine/internal/registry"
)

// detectUSB detects USB printers using libusb
func (m *Manager) detectUSB() ([]*Printer, error) {
	var printers []*Printer
	
	// Initialize USB context
	ctx := gousb.NewContext()
	defer ctx.Close()
	
	// Find all USB devices
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// Accept all devices - we'll filter later
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate USB devices: %w", err)
	}
	
	for _, dev := range devices {
		desc, err := dev.DeviceDesc()
		if err != nil {
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
