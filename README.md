# Receipt Engine

A high-performance, cross-platform printer driver server for thermal receipt printers, written in Go.

## Features

- 🔌 **Multi-Interface Support**: USB, Serial (RS232), and Network (Ethernet/WiFi) printers
- 🎨 **High-Fidelity Rendering**: Pixel-perfect image-based rendering using custom fonts
- 📡 **WebSocket & HTTP API**: Real-time printer detection and command execution
- 🖨️ **Rich Command Set**: Text, images, barcodes, QR codes, layouts, and more
- 🔄 **Hot-Plug Detection**: Automatic printer discovery and reconnection
- 📝 **`.receipt` Format**: JSON-based receipt template system with variables and arrays

## Architecture

```
receipt-engine/
├── cmd/server/          # Main entry point
├── internal/
│   ├── printer/         # Printer abstraction & detection
│   ├── parser/          # Command parser
│   ├── renderer/        # Image rendering engine
│   ├── api/             # HTTP/WebSocket handlers
│   └── registry/        # Printer registry (persistent IDs)
└── pkg/
    └── receiptformat/   # Public types for .receipt format
```

## Quick Start

### Prerequisites

- Go 1.21+
- libusb (for USB printer support)
  - **macOS**: `brew install libusb`
  - **Linux**: `apt-get install libusb-1.0-0-dev`
  - **Windows**: Install WinUSB drivers

### Installation

```bash
go mod download
go build -o receipt-engine ./cmd/server
./receipt-engine
```

The server will start on `http://localhost:12212`.

### API Endpoints

- `GET /printers` - List all detected printers
- `POST /printer/:id/name` - Set custom printer name
- `WS /` - WebSocket endpoint for print commands

### WebSocket Events

- `print` - Send print commands
- `printer_added` - Printer connected
- `printer_removed` - Printer disconnected

## Development Status

**Phase 1: Project Setup** ✅ (Current)
- [x] Project structure
- [x] Go module initialization
- [ ] CI/CD setup
- [ ] Cross-compilation

**Phase 2: Receipt Format Schema** (Next)
- [ ] Schema definition
- [ ] JSON marshaling
- [ ] Validation

See [go_engine_plan.md](../go_engine_plan.md) for the full roadmap.

## Contributing

This project follows the Receipt ecosystem architecture. See:
- [Product Architecture](../product_architecture.md)
- [Receipt Format Specification](../receipt_file_format/RECEIPT_FORMAT.md)

## License

MIT License - See LICENSE file for details
