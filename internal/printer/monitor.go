package printer

import (
	"context"
	"fmt"
	"time"
)

// Monitor continuously monitors for printer changes
type Monitor struct {
	manager  *Manager
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewMonitor creates a new printer monitor
func NewMonitor(manager *Manager, interval time.Duration) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Monitor{
		manager:  manager,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins monitoring for printer changes
func (m *Monitor) Start() {
	// Store initial state
	previousPrinters := make(map[string]*Printer)
	
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.checkChanges(previousPrinters)
			}
		}
	}()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	m.cancel()
}

func (m *Monitor) checkChanges(previousPrinters map[string]*Printer) {
	// Detect current printers
	currentPrinters, err := m.manager.DetectPrinters()
	if err != nil {
		fmt.Printf("Warning: printer detection failed: %v\n", err)
		return
	}
	
	// Build current map
	currentMap := make(map[string]*Printer)
	for _, p := range currentPrinters {
		currentMap[p.ID] = p
	}
	
	// Find new printers
	for id, printer := range currentMap {
		if _, exists := previousPrinters[id]; !exists {
			// New printer detected
			fmt.Printf("🟢 Printer added: %s\n", printer.Description)
			if m.manager.onPrinterAdded != nil {
				m.manager.onPrinterAdded(printer)
			}
		}
	}
	
	// Find removed printers
	for id, printer := range previousPrinters {
		if _, exists := currentMap[id]; !exists {
			// Printer removed
			fmt.Printf("🔴 Printer removed: %s\n", printer.Description)
			if m.manager.onPrinterRemoved != nil {
				m.manager.onPrinterRemoved(id)
			}
		}
	}
	
	// Update previous state
	for id := range previousPrinters {
		delete(previousPrinters, id)
	}
	for id, p := range currentMap {
		previousPrinters[id] = p
	}
}
