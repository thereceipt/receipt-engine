package registry

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	// Create temp file for testing
	tmpFile := "/tmp/test_registry.json"
	defer os.Remove(tmpFile)
	
	reg, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	
	if reg == nil {
		t.Fatal("Registry is nil")
	}
}

func TestGetPrinterID_USB(t *testing.T) {
	tmpFile := "/tmp/test_registry_usb.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "usb",
		VID:         0x04B8,
		PID:         0x0E15,
		Description: "Epson TM-T20",
	}
	
	// First call should create new ID
	id1 := reg.GetPrinterID(info)
	if id1 == "" {
		t.Error("Expected non-empty printer ID")
	}
	
	// Second call with same info should return same ID
	id2 := reg.GetPrinterID(info)
	if id1 != id2 {
		t.Errorf("Expected same ID for same printer: %s != %s", id1, id2)
	}
}

func TestGetPrinterID_Serial(t *testing.T) {
	tmpFile := "/tmp/test_registry_serial.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "serial",
		Device:      "/dev/cu.usbserial-1234",
		Description: "Serial Printer",
	}
	
	id := reg.GetPrinterID(info)
	if id == "" {
		t.Error("Expected non-empty printer ID")
	}
}

func TestGetPrinterID_Network(t *testing.T) {
	tmpFile := "/tmp/test_registry_network.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "network",
		Host:        "192.168.1.100",
		Port:        9100,
		Description: "Network Printer",
	}
	
	id := reg.GetPrinterID(info)
	if id == "" {
		t.Error("Expected non-empty printer ID")
	}
}

func TestSetAndGetPrinterName(t *testing.T) {
	tmpFile := "/tmp/test_registry_name.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "usb",
		VID:         0x04B8,
		PID:         0x0E15,
		Description: "Test Printer",
	}
	
	id := reg.GetPrinterID(info)
	
	// Set custom name
	success := reg.SetPrinterName(id, "Kitchen Printer")
	if !success {
		t.Error("Expected successful name set")
	}
	
	// Get custom name
	name := reg.GetPrinterName(id)
	if name != "Kitchen Printer" {
		t.Errorf("Expected 'Kitchen Printer', got '%s'", name)
	}
}

func TestGetPrinterInfo(t *testing.T) {
	tmpFile := "/tmp/test_registry_info.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "usb",
		VID:         0x04B8,
		PID:         0x0E15,
		Description: "Test Printer",
	}
	
	id := reg.GetPrinterID(info)
	reg.SetPrinterName(id, "Front Counter")
	
	entry := reg.GetPrinterInfo(id)
	if entry == nil {
		t.Fatal("Expected printer info, got nil")
	}
	
	if entry.Type != "usb" {
		t.Errorf("Expected type 'usb', got '%s'", entry.Type)
	}
	if entry.VID != 0x04B8 {
		t.Errorf("Expected VID 0x04B8, got 0x%04X", entry.VID)
	}
	if entry.Name != "Front Counter" {
		t.Errorf("Expected name 'Front Counter', got '%s'", entry.Name)
	}
}

func TestRemovePrinter(t *testing.T) {
	tmpFile := "/tmp/test_registry_remove.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	info := PrinterInfo{
		Type:        "usb",
		VID:         0x1234,
		PID:         0x5678,
		Description: "Test",
	}
	
	id := reg.GetPrinterID(info)
	
	// Remove printer
	success := reg.RemovePrinter(id)
	if !success {
		t.Error("Expected successful removal")
	}
	
	// Try to get info - should return nil
	entry := reg.GetPrinterInfo(id)
	if entry != nil {
		t.Error("Expected nil after removal")
	}
}

func TestPersistence(t *testing.T) {
	tmpFile := "/tmp/test_registry_persist.json"
	defer os.Remove(tmpFile)
	
	// Create registry and add printer
	reg1, _ := New(tmpFile)
	info := PrinterInfo{
		Type:        "usb",
		VID:         0xAAAA,
		PID:         0xBBBB,
		Description: "Persistent Printer",
	}
	id1 := reg1.GetPrinterID(info)
	reg1.SetPrinterName(id1, "Persistent Name")
	
	// Create new registry instance (simulating app restart)
	reg2, _ := New(tmpFile)
	
	// Should get same ID
	id2 := reg2.GetPrinterID(info)
	if id1 != id2 {
		t.Errorf("Expected same ID after reload: %s != %s", id1, id2)
	}
	
	// Should have same name
	name := reg2.GetPrinterName(id2)
	if name != "Persistent Name" {
		t.Errorf("Expected name to persist, got '%s'", name)
	}
}

func TestGetAll(t *testing.T) {
	tmpFile := "/tmp/test_registry_getall.json"
	defer os.Remove(tmpFile)
	
	reg, _ := New(tmpFile)
	
	// Add multiple printers
	info1 := PrinterInfo{Type: "usb", VID: 0x1111, PID: 0x2222, Description: "Printer 1"}
	info2 := PrinterInfo{Type: "serial", Device: "/dev/tty1", Description: "Printer 2"}
	
	reg.GetPrinterID(info1)
	reg.GetPrinterID(info2)
	
	all := reg.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 printers, got %d", len(all))
	}
}
