# Receipt Engine

A high-performance, cross-platform thermal printer server written in Go. Supports USB, Serial, and Network printers with a rich JSON-based receipt format.

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+**
- **C compiler** (for serial port support on Unix systems)
  - macOS: Xcode Command Line Tools (`xcode-select --install`)
  - Linux: `sudo apt-get install gcc`
  - Windows: Included with MSYS2/MinGW
**Note**: USB support is statically linked into the binaries - no separate installation needed! USB printers work out of the box.

### Installation

```bash
# Clone the repository
git clone https://github.com/thereceipt/receipt-engine.git
cd receipt-engine

# Install dependencies
go mod download

# Run the server
go run ./cmd/server
```

The server will start on `http://localhost:12212`

## 📋 Features

- ✅ **Multi-Interface Support**: USB (direct), Serial (RS232/USB-to-Serial), Network (TCP/IP)
- ✅ **Rich Receipt Format**: JSON-based `.receipt` files with variables and arrays
- ✅ **Image Rendering**: High-fidelity rendering with custom fonts
- ✅ **ESC/POS Support**: Industry-standard thermal printer commands
- ✅ **HTTP & WebSocket API**: RESTful and real-time interfaces
- ✅ **Hot-Plug Detection**: Automatic printer discovery
- ✅ **Print Queue**: Retry logic and job tracking
- ✅ **Template Variables**: Dynamic receipt content
- ✅ **Variable Arrays**: Render lists (products, items, etc.)

## 🖨️ Supported Printers

Any ESC/POS compatible thermal printer:
- Epson TM-T20, TM-T88
- Star Micronics TSP100, TSP650
- Zebra ZD410, ZD420
- And many more...

## 📡 API Reference

### HTTP Endpoints

```bash
GET  /printers           # List all detected printers
POST /printer/:id/name   # Set custom printer name
POST /print              # Print a receipt
GET  /jobs               # List all print jobs
GET  /job/:id            # Get job status
GET  /health             # Health check
```

### WebSocket

Connect to `ws://localhost:12212/ws`

**Events:**
- `print` - Send print job
- `printer_added` - Printer connected (server → client)
- `printer_removed` - Printer disconnected (server → client)

## 📄 Receipt Format

Receipts are defined in JSON:

```json
{
  "version": "1.0",
  "paper_width": "80mm",
  "commands": [
    {
      "type": "text",
      "value": "Hello World",
      "size": 24,
      "align": "center"
    },
    {
      "type": "qrcode",
      "value": "https://example.com"
    },
    {
      "type": "cut"
    }
  ]
}
```

### Command Types

- `text` - Formatted text
- `image` - Images (file path or base64)
- `barcode` - 1D barcodes (CODE128, EAN13, etc.)
- `qrcode` - QR codes
- `item` - Two-column layout (product lists)
- `box` - Bordered containers
- `divider` - Horizontal lines
- `feed` - Paper feed
- `cut` - Paper cut

### Template Variables

```json
{
  "variables": [
    {
      "let": "total",
      "valueType": "double",
      "defaultValue": 0.0,
      "prefix": "$"
    }
  ],
  "commands": [
    {
      "type": "text",
      "dynamicValue": "total"
    }
  ]
}
```

### Variable Arrays

```json
{
  "variableArrays": [
    {
      "name": "products",
      "schema": [
        {"field": "name", "valueType": "string"},
        {"field": "price", "valueType": "double", "prefix": "$"}
      ]
    }
  ],
  "commands": [
    {
      "type": "item",
      "arrayBinding": "products",
      "left_side": [{"type": "text", "arrayField": "name"}],
      "right_side": [{"type": "text", "arrayField": "price"}]
    }
  ]
}
```

## 🔧 Usage Examples

### Print a Simple Receipt

```bash
curl -X POST http://localhost:12212/print \
  -H "Content-Type: application/json" \
  -d '{
    "printer_id": "your-printer-id",
    "receipt": {
      "version": "1.0",
      "commands": [
        {"type": "text", "value": "Hello World", "size": 24},
        {"type": "feed", "lines": 2},
        {"type": "cut"}
      ]
    }
  }'
```

### Print with Variables

```bash
curl -X POST http://localhost:12212/print \
  -H "Content-Type: application/json" \
  -d '{
    "printer_id": "your-printer-id",
    "receipt": {
      "version": "1.0",
      "variables": [
        {"let": "storeName", "valueType": "string"},
        {"let": "total", "valueType": "double", "prefix": "$"}
      ],
      "commands": [
        {"type": "text", "dynamicValue": "storeName"},
        {"type": "text", "dynamicValue": "total"}
      ]
    },
    "variableData": {
      "storeName": "Coffee Shop",
      "total": 25.99
    }
  }'
```

### WebSocket Example

```javascript
const ws = new WebSocket('ws://localhost:12212/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    event: 'print',
    data: {
      printer_id: 'your-printer-id',
      receipt: {
        version: '1.0',
        commands: [
          {type: 'text', value: 'Hello from WebSocket!'},
          {type: 'cut'}
        ]
      }
    }
  }));
};

ws.onmessage = (event) => {
  const response = JSON.parse(event.data);
  console.log('Print job:', response.data.job_id);
};
```

## 🏗️ Architecture

```
receipt-engine/
├── cmd/server/          # Main entry point
├── internal/
│   ├── api/             # HTTP/WebSocket handlers
│   ├── parser/          # Command parser (variables/arrays)
│   ├── printer/         # Hardware detection & communication
│   ├── renderer/        # Image rendering
│   └── registry/        # Persistent printer IDs
└── pkg/
    └── receiptformat/   # Public schema types
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Test with coverage
go test -cover ./...

# Test specific package
go test ./internal/renderer
```

## 📦 Building

```bash
# Build for current platform
go build -o receipt-engine ./cmd/server

# Cross-compile for all platforms
make cross-compile
```

Outputs will be in `dist/`:
- `receipt-engine-darwin-arm64` (macOS Apple Silicon)
- `receipt-engine-darwin-amd64` (macOS Intel)
- `receipt-engine-windows-amd64.exe` (Windows 64-bit)
- `receipt-engine-linux-amd64` (Linux 64-bit)
- `receipt-engine-linux-arm64` (Linux ARM64)

## 🐳 Docker (Coming Soon)

```bash
docker build -t receipt-engine .
docker run -p 12212:12212 --privileged receipt-engine
```

## 📚 Documentation

- [Receipt Format Specification](../receipt_file_format/RECEIPT_FORMAT.md)
- [Variable Arrays Guide](../receipt_file_format/VARIABLE_ARRAYS.md)
- [Product Architecture](../.gemini/antigravity/brain/*/product_architecture.md)
- [NPM Package Specs](../.gemini/antigravity/brain/*/npm_packages_spec.md)

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) first.

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details

## 🙏 Acknowledgments

Built with:
- [Gin](https://github.com/gin-gonic/gin) - HTTP framework
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - WebSocket support
- [gousb](https://github.com/google/gousb) - USB device access (optional, graceful degradation)
- [tarm/serial](https://github.com/tarm/serial) - Serial port access
- [gg](https://github.com/fogleman/gg) - 2D rendering
- [go-qrcode](https://github.com/skip2/go-qrcode) - QR code generation

## 💬 Support

- GitHub Issues: [Report bugs or request features](https://github.com/thereceipt/receipt-engine/issues)
- Documentation: [Full docs](https://github.com/thereceipt/receipt-engine/wiki)

---

**Made with ❤️ for thermal printers everywhere**
