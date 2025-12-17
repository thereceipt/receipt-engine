# Command System

The Receipt Engine now includes a powerful command system that allows you to interact with the engine using clean, intuitive commands. The command system is available in three ways:

1. **TUI Command Bar** - Press `:` in the TUI to open the command bar
2. **API Endpoint** - POST commands to `/command`
3. **Standalone CLI** - Use the `receipt-cli` tool

## Architecture

The command system is modular and cleanly separated:

- **`internal/command/`** - Core command executor and handlers
- **`cmd/cli/`** - Standalone CLI tool that communicates with the API
- **TUI Integration** - Command bar integrated into the TUI

## Available Commands

### Print Commands

```bash
print <printer-id> <receipt-path> [--var key=value] [--var-array key=value1,value2]
```

Print a receipt to the specified printer.

**Examples:**
```bash
print printer-123 ./receipt.receipt
print printer-123 ./receipt.receipt --var customer="John Doe"
print printer-123 ./receipt.receipt --var customer="John" --var total=25.50
```

### Printer Commands

```bash
printer list
printer add-network <host> [port]
printer rename <id> <name>
```

Manage printers.

**Examples:**
```bash
printer list
printer add-network 192.168.1.100 9100
printer rename printer-123 "Kitchen Printer"
```

### Job Commands

```bash
job list
job status <id>
job clear
```

Manage print jobs.

**Examples:**
```bash
job list
job status job-456
job clear
```

### Utility Commands

```bash
detect
help
```

Detect printers or show help.

## Usage

### TUI Command Bar

1. Press `:` to open the command bar
2. Type your command
3. Press `Enter` to execute
4. Press `Esc` to close

The command bar shows results inline and stays open for quick commands.

### API Endpoint

**POST** `/command`

**Request:**
```json
{
  "command": "printer list"
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Found 2 printer(s)",
  "printers": [...]
}
```

**Response (Error):**
```json
{
  "success": false,
  "error": "printer not found: printer-123"
}
```

### Standalone CLI

Build the CLI:
```bash
go build -o receipt-cli ./cmd/cli
```

Use it:
```bash
# Default server (localhost:12212)
./receipt-cli printer list

# Custom server
./receipt-cli -s http://localhost:8080 printer list

# Print a receipt
./receipt-cli print printer-123 ./receipt.receipt
```

## Command Syntax

Commands follow a simple, intuitive syntax:

- **Space-separated arguments**: `printer list`
- **Quoted strings**: `printer rename printer-123 "My Printer"`
- **Flags**: `--var key=value` for variable data
- **Subcommands**: `printer list`, `job status`

## Examples

### Complete Workflow

```bash
# 1. Detect printers
detect

# 2. List printers
printer list

# 3. Add a network printer
printer add-network 192.168.1.100 9100

# 4. Rename a printer
printer rename printer-123 "Kitchen Printer"

# 5. Print a receipt
print printer-123 ./receipt.receipt --var customer="John Doe"

# 6. Check job status
job status job-456

# 7. List all jobs
job list

# 8. Clear completed jobs
job clear
```

## Integration

The command system is fully integrated:

- **TUI**: Press `:` for quick commands
- **API**: Use `/command` endpoint for programmatic access
- **CLI**: Use `receipt-cli` for shell scripts and automation

All three interfaces use the same command executor, ensuring consistent behavior across all access methods.
