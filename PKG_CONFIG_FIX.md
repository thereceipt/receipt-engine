# Quick Fix for Missing pkg-config

## The Issue
`github.com/google/gousb` requires `pkg-config` and `libusb` to compile USB support.

## Solution Options

### Option 1: Install pkg-config (Recommended)
```bash
brew install pkg-config libusb
```

Then run:
```bash
go run ./cmd/server
```

### Option 2: Build without USB support (Quick Start)
Comment out USB detection temporarily in `internal/printer/manager.go`:

```go
func (m *Manager) detectUSB() ([]*Printer, error) {
    // Temporarily disabled - install pkg-config and libusb to enable
    return []*Printer{}, nil
}
```

This allows testing with Serial and Network printers while you install dependencies.

### Option 3: Use the Pre-built Binary
If available, use a pre-compiled binary that doesn't require pkg-config at runtime.

## After Installing pkg-config

Run the server:
```bash
go run ./cmd/server
```

You should see:
- ✅ Printer detection working
- ✅ Server running on http://localhost:12212
