package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/thereceipt/receipt-engine/internal/api"
	"github.com/thereceipt/receipt-engine/internal/printer"
	"github.com/thereceipt/receipt-engine/internal/tui"
)

// Version is set during build via ldflags
var Version = "dev"

func main() {
	port := getPort()
	registryPath := getRegistryPath()

	// Initialize printer manager
	manager, err := printer.NewManager(registryPath)
	if err != nil {
		log.Fatalf("Failed to create printer manager: %v", err)
	}

	// Detect printers
	printers, err := manager.DetectPrinters()
	if err != nil {
		log.Printf("Warning: printer detection failed: %v", err)
	}

	// Create connection pool
	pool := printer.NewConnectionPool()

	// Create print queue with 3 retries
	queue := printer.NewPrintQueue(pool, manager, 3)
	defer queue.Stop()

	// Start printer monitor
	monitor := printer.NewMonitor(manager, 2*time.Second)

	// Create TUI app (using tview)
	tuiApp := tui.NewTViewApp(manager, pool, queue, port)

	// Set up log capture to TUI
	logWriter := tuiApp.LogWriter()
	log.SetOutput(io.MultiWriter(os.Stderr, logWriter))

	// Set up printer event callbacks to log to TUI
	manager.OnPrinterAdded(func(p *printer.Printer) {
		name := p.Description
		if p.Name != "" {
			name = p.Name
		}
		tuiApp.AddLog(fmt.Sprintf("🟢 Printer connected: %s", name), "info")
		// Refresh printers panel
		tuiApp.App.QueueUpdateDraw(func() {
			tuiApp.RefreshPrinters()
		})
	})

	manager.OnPrinterRemoved(func(id string) {
		tuiApp.AddLog(fmt.Sprintf("🔴 Printer disconnected: %s", id), "info")
		// Refresh printers panel
		tuiApp.App.QueueUpdateDraw(func() {
			tuiApp.RefreshPrinters()
		})
	})

	monitor.Start()
	defer monitor.Stop()

	// Create API server
	server := api.NewServer(manager, pool, queue)

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		tuiApp.AddLog(fmt.Sprintf("🚀 Starting API server on %s", addr), "info")
		if err := server.Run(addr); err != nil {
			serverErrChan <- err
		}
	}()

	// Start TUI
	tuiApp.AddLog("🖨️  Receipt Engine starting...", "info")
	if len(printers) > 0 {
		tuiApp.AddLog(fmt.Sprintf("✅ Found %d printer(s)", len(printers)), "info")
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Run TUI (blocking)
	tuiDone := make(chan struct{})
	go func() {
		if err := tuiApp.Run(); err != nil {
			log.Printf("TUI error: %v", err)
		}
		close(tuiDone)
	}()

	// Wait for either TUI to quit, server error, or signal
	select {
	case err := <-serverErrChan:
		log.Fatalf("Server error: %v", err)
	case <-sigChan:
		// Signal received, shutdown gracefully
		tuiApp.AddLog("🛑 Shutting down...", "info")
		pool.DisconnectAll()
		os.Exit(0)
	case <-tuiDone:
		// TUI quit, shutdown gracefully
		pool.DisconnectAll()
		os.Exit(0)
	}
}

func getPort() string {
	if port := os.Getenv("SERVER_PORT"); port != "" {
		return port
	}

	// Check command line args
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}

	return "12212"
}

// getRegistryPath returns the path to the printer registry file.
// It tries to place it next to the executable, or falls back to current directory.
func getRegistryPath() string {
	// First, try to get the executable path and place registry next to it
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		registryPath := filepath.Join(exeDir, "printer_registry.json")

		// Check if we can write to the executable directory
		if testDir := exeDir; testDir != "" {
			if info, err := os.Stat(testDir); err == nil && info.IsDir() {
				// Try to create a test file to check write permissions
				testFile := filepath.Join(testDir, ".receipt-engine-write-test")
				if f, err := os.Create(testFile); err == nil {
					f.Close()
					os.Remove(testFile)
					return registryPath
				}
			}
		}
	}

	// Fallback: use current directory
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "printer_registry.json")
	}

	// Last resort: use home directory config (Unix) or AppData (Windows)
	var configDir string
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			configDir = filepath.Join(appData, "receipt-engine")
		} else {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "receipt-engine")
		}
	} else {
		// Unix-like systems
		if home := os.Getenv("HOME"); home != "" {
			configDir = filepath.Join(home, ".config", "receipt-engine")
		}
	}

	if configDir != "" {
		// Create directory if it doesn't exist
		os.MkdirAll(configDir, 0755)
		return filepath.Join(configDir, "printer_registry.json")
	}

	// Absolute last resort: current directory (shouldn't reach here)
	return "printer_registry.json"
}
