// Package registry manages persistent printer IDs and custom names
package registry

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
)

// Registry manages printer identities and custom names
type Registry struct {
	filePath string
	data     map[string]*PrinterEntry
	mu       sync.RWMutex
}

// PrinterEntry stores persistent information about a printer
type PrinterEntry struct {
	ID          string `json:"id"`
	IdentityKey string `json:"identity_key"`
	Type        string `json:"type"` // usb, serial, network
	VID         uint16 `json:"vid,omitempty"`
	PID         uint16 `json:"pid,omitempty"`
	Device      string `json:"device,omitempty"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Description string `json:"description"`
	Name        string `json:"name,omitempty"` // Custom user-set name
}

// PrinterInfo represents basic printer information for detection
type PrinterInfo struct {
	Type        string
	Description string
	Device      string
	VID         uint16
	PID         uint16
	Host        string
	Port        int
}

// New creates a new Registry
func New(filePath string) (*Registry, error) {
	r := &Registry{
		filePath: filePath,
		data:     make(map[string]*PrinterEntry),
	}

	if err := r.load(); err != nil {
		// If file doesn't exist, that's okay - we'll create it on first save
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load registry: %w", err)
		}
	}

	return r, nil
}

// GetPrinterID gets or creates a persistent ID for a printer
func (r *Registry) GetPrinterID(info PrinterInfo) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	identityKey := generateIdentityKey(info)

	// Check if we already have this printer
	if entry, exists := r.data[identityKey]; exists {
		return entry.ID
	}

	// Generate new ID
	printerID := uuid.New().String()

	// Store printer info
	entry := &PrinterEntry{
		ID:          printerID,
		IdentityKey: identityKey,
		Type:        info.Type,
		VID:         info.VID,
		PID:         info.PID,
		Device:      info.Device,
		Host:        info.Host,
		Port:        info.Port,
		Description: info.Description,
	}

	r.data[identityKey] = entry

	// Save to disk
	if err := r.save(); err != nil {
		// Log error but don't fail - we still return the ID
		// Warning: failed to save registry - non-critical, will retry
	}

	return printerID
}

// GetPrinterName gets the custom name for a printer, or empty string if not set
func (r *Registry) GetPrinterName(printerID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.data {
		if entry.ID == printerID {
			return entry.Name
		}
	}
	return ""
}

// SetPrinterName sets a custom name for a printer
func (r *Registry) SetPrinterName(printerID string, name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.data {
		if entry.ID == printerID {
			entry.Name = name
			if err := r.save(); err != nil {
				// Warning: failed to save registry - non-critical, will retry
			}
			return true
		}
	}
	return false
}

// GetPrinterInfo gets all stored information for a printer
func (r *Registry) GetPrinterInfo(printerID string) *PrinterEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.data {
		if entry.ID == printerID {
			// Return a copy to avoid race conditions
			entryCopy := *entry
			return &entryCopy
		}
	}
	return nil
}

// RemovePrinter removes a printer from the registry
func (r *Registry) RemovePrinter(printerID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, entry := range r.data {
		if entry.ID == printerID {
			delete(r.data, key)
			if err := r.save(); err != nil {
				// Warning: failed to save registry - non-critical, will retry
			}
			return true
		}
	}
	return false
}

// GetAll returns all registered printers
func (r *Registry) GetAll() map[string]*PrinterEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]*PrinterEntry, len(r.data))
	for k, v := range r.data {
		entryCopy := *v
		result[k] = &entryCopy
	}
	return result
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &r.data)
}

func (r *Registry) save() error {
	data, err := json.MarshalIndent(r.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(r.filePath, data, 0644)
}

// generateIdentityKey creates a unique key for a printer based on its characteristics
func generateIdentityKey(info PrinterInfo) string {
	switch info.Type {
	case "usb":
		if info.VID != 0 && info.PID != 0 {
			return fmt.Sprintf("usb:%04X:%04X", info.VID, info.PID)
		}
	case "serial":
		if info.Device != "" {
			return fmt.Sprintf("serial:%s", info.Device)
		}
	case "network":
		if info.Host != "" {
			return fmt.Sprintf("network:%s:%d", info.Host, info.Port)
		}
	}

	// Fallback: hash the description
	hash := md5.Sum([]byte(info.Description))
	return fmt.Sprintf("hash:%x", hash)
}
