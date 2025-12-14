# Deployment Guide

## Production Deployment

### Option 1: Systemd Service (Linux)

Create `/etc/systemd/system/receipt-engine.service`:

```ini
[Unit]
Description=Receipt Engine Server
After=network.target

[Service]
Type=simple
User=receipt
WorkingDirectory=/opt/receipt-engine
ExecStart=/opt/receipt-engine/receipt-engine
Restart=always
RestartSec=10

# Environment
Environment="SERVER_PORT=12212"

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable receipt-engine
sudo systemctl start receipt-engine
sudo systemctl status receipt-engine
```

### Option 2: Docker

**Dockerfile:**
```dockerfile
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git libusb-dev gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o receipt-engine ./cmd/server

FROM alpine:latest
RUN apk add --no-cache libusb

COPY --from=builder /app/receipt-engine /usr/local/bin/

EXPOSE 12212

CMD ["receipt-engine"]
```

**Build and run:**
```bash
docker build -t receipt-engine .
docker run -d \
  --name receipt-engine \
  --privileged \
  -p 12212:12212 \
  -v /dev:/dev \
  receipt-engine
```

### Option 3: Binary Deployment

```bash
# Build
make cross-compile

# Deploy to server
scp dist/receipt-engine-linux-amd64 user@server:/usr/local/bin/receipt-engine

# Create service user
sudo useradd -r -s /bin/false receipt

# Set up systemd (see Option 1)
```

## Environment Variables

```bash
SERVER_PORT=12212              # API server port
REGISTRY_PATH=registry.json    # Printer registry file path
MAX_RETRIES=3                  # Print job retry count
MONITOR_INTERVAL=2s            # Printer detection interval
```

## Reverse Proxy (nginx)

```nginx
server {
    listen 80;
    server_name receipts.example.com;

    location / {
        proxy_pass http://localhost:12212;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    location /ws {
        proxy_pass http://localhost:12212/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Security

### Firewall Rules

```bash
# Allow only local connections
sudo ufw allow from 127.0.0.1 to any port 12212

# Or allow specific subnet
sudo ufw allow from 192.168.1.0/24 to any port 12212
```

### Authentication (TODO)

Add API key authentication:
```bash
export API_KEY="your-secret-key"
```

## Monitoring

### Health Check

```bash
curl http://localhost:12212/health
```

### Logs

```bash
# Systemd
journalctl -u receipt-engine -f

# Docker
docker logs -f receipt-engine
```

### Metrics (TODO)

Prometheus endpoint: `/metrics`

## Backup

Backup the printer registry:
```bash
cp printer_registry.json printer_registry.backup.json
```

## Troubleshooting

### USB Permissions

```bash
# Add user to dialout group
sudo usermod -a -G dialout $USER

# Create udev rule for USB printers
echo 'SUBSYSTEM=="usb", MODE="0666"' | sudo tee /etc/udev/rules.d/99-usb-printers.rules
sudo udevadm control --reload-rules
```

### Serial Port Access

```bash
# List serial devices
ls -l /dev/tty*

# Grant permissions
sudo chmod 666 /dev/ttyUSB0
```

### Network Printer Not Responding

```bash
# Test connectivity
telnet printer-ip 9100

# Check firewall
sudo iptables -L
```

## Performance Tuning

### Increase Queue Workers

Modify `internal/printer/queue.go` to add more workers.

### Adjust Retry Logic

Set `MAX_RETRIES` environment variable.

### Connection Pooling

Connection pool automatically manages printer connections.
