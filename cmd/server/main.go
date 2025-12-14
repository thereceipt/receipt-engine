package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"github.com/thereceipt/receipt-engine/internal/api"
	"github.com/thereceipt/receipt-engine/internal/printer"
)

func main() {
	port := getPort()
	
	fmt.Printf("🖨️  Receipt Engine Server\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	
	// Initialize printer manager
	fmt.Println("📋 Initializing printer manager...")
	manager, err := printer.NewManager("printer_registry.json")
	if err != nil {
		log.Fatalf("Failed to create printer manager: %v", err)
	}
	
	// Detect printers
	fmt.Println("🔍 Detecting printers...")
	printers, err := manager.DetectPrinters()
	if err != nil {
		log.Printf("Warning: printer detection failed: %v", err)
	} else {
		fmt.Printf("✅ Found %d printer(s)\n", len(printers))
		for _, p := range printers {
			name := p.Description
			if p.Name != "" {
				name = p.Name
			}
			fmt.Printf("   • %s [%s]\n", name, p.Type)
		}
	}
	
	// Create connection pool
	pool := printer.NewConnectionPool()
	
	// Create print queue with 3 retries
	queue := printer.NewPrintQueue(pool, manager, 3)
	defer queue.Stop()
	
	// Start printer monitor
	fmt.Println("👁️  Starting printer monitor...")
	monitor := printer.NewMonitor(manager, 2*time.Second)
	
	manager.OnPrinterAdded(func(p *printer.Printer) {
		name := p.Description
		if p.Name != "" {
			name = p.Name
		}
		fmt.Printf("🟢 Printer connected: %s\n", name)
	})
	
	manager.OnPrinterRemoved(func(id string) {
		fmt.Printf("🔴 Printer disconnected: %s\n", id)
	})
	
	monitor.Start()
	defer monitor.Stop()
	
	// Create API server
	fmt.Println("🚀 Starting API server...")
	server := api.NewServer(manager, pool, queue)
	
	// Start server in goroutine
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", port)
		fmt.Printf("\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("✅ Server ready on http://localhost:%s\n", port)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
		fmt.Printf("API Endpoints:\n")
		fmt.Printf("  GET  /printers           - List all printers\n")
		fmt.Printf("  POST /printer/:id/name   - Set printer name\n")
		fmt.Printf("  POST /print              - Print receipt\n")
		fmt.Printf("  GET  /jobs               - List print jobs\n")
		fmt.Printf("  GET  /job/:id            - Get job status\n")
		fmt.Printf("  GET  /ws                 - WebSocket endpoint\n")
		fmt.Printf("  GET  /health             - Health check\n")
		fmt.Printf("\n")
		
		if err := server.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	
	fmt.Println("\n\n🛑 Shutting down...")
	pool.DisconnectAll()
	fmt.Println("✅ Cleanup complete")
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
