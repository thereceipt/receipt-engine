# Network Printer Detection - Setup Guide

## What You NEED to Check:

### 1. **Printer Network Configuration** ✅
Your printer MUST be:
- ✅ Connected to the **same network** (same WiFi/router) as your computer
- ✅ Has a **static IP address** OR you know its current IP address
- ✅ Has **network printing enabled** (check printer settings)
- ✅ Is **powered on** and connected to the network

### 2. **Printer Port Requirements** 🔌
The scanner looks for printers on these ports:
- **Port 9100** (Raw TCP/IP printing) - **MOST COMMON for receipt printers**
- **Port 515** (LPR printing)

**Your printer MUST accept TCP connections on port 9100** (or 515).

### 3. **Find Your Printer's IP Address** 📍

**Option A: Check Printer Display**
- Many printers show their IP on the display/settings menu
- Look for "Network Settings" → "TCP/IP" → "IP Address"

**Option B: Check Your Router**
- Log into your router admin panel (usually 192.168.1.1 or 192.168.0.1)
- Look at "Connected Devices" or "DHCP Client List"
- Find your printer by name or MAC address

**Option C: Use Terminal Commands**
```bash
# On macOS/Linux - scan your network
nmap -p 9100 192.168.1.0/24

# Or check ARP table
arp -a | grep -i printer

# Or ping your router's subnet
for i in {1..254}; do
  timeout 0.1 ping -c 1 192.168.1.$i 2>&1 | grep "64 bytes" &
done
```

**Option D: Check Printer Configuration Page**
- Open printer's web interface: `http://[printer-ip]` in browser
- Look for "Network" or "TCP/IP" settings

### 4. **Verify Printer is Reachable** 🔍

Test if your printer accepts connections:

```bash
# Test port 9100 (most common)
nc -zv [PRINTER_IP] 9100

# Or use telnet
telnet [PRINTER_IP] 9100

# If connection succeeds, you'll see "Connected" or the connection opens
```

**If connection fails:**
- Printer might not support raw TCP printing
- Firewall might be blocking
- Printer might use a different port

### 5. **Check Scanner Limitations** ⚠️

The automatic scanner:
- ✅ Scans first **100 IPs** in each subnet
- ✅ Only checks ports **9100** and **515**
- ✅ Only scans **active network interfaces**
- ⚠️ **Skips IPv6** (only IPv4)
- ⚠️ **200ms timeout** per connection (might miss slow printers)

**If your printer IP is > 100 in the subnet, it won't be auto-detected!**

### 6. **Manual Addition (Recommended)** 🎯

If automatic detection doesn't work, **manually add your printer**:

```bash
curl -X POST http://localhost:12212/printer/network \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.100",
    "port": 9100,
    "description": "My Receipt Printer"
  }'
```

Replace:
- `192.168.1.100` with your printer's actual IP
- `9100` with the correct port (try 9100 first, then 515)

### 7. **Check Server Logs** 📋

When you start the server, look for these messages:

```
🔍 Starting background network printer scan...
🔍 Scanning X network interface(s) for printers on ports [9100 515]...
✅ Found network printer at X.X.X.X:9100
🟢 Added network printer: Network: X.X.X.X:9100
```

**If you don't see "Found network printer" messages:**
- Printer might not be on the scanned IP range
- Printer might not be listening on ports 9100/515
- Firewall might be blocking

### 8. **Common Issues & Solutions** 🔧

| Problem | Solution |
|---------|----------|
| Printer not detected | Manually add using API (see #6) |
| Connection timeout | Check printer IP, verify it's on same network |
| Port not open | Check printer supports raw TCP (port 9100) |
| Printer IP > 100 | Manually add - scanner only checks first 100 IPs |
| Firewall blocking | Temporarily disable firewall to test |
| Wrong port | Try port 515 if 9100 doesn't work |

### 9. **Quick Test Checklist** ✅

Before running the server:
- [ ] Printer is powered on
- [ ] Printer is connected to same WiFi/network as your computer
- [ ] You know the printer's IP address
- [ ] You can ping the printer: `ping [PRINTER_IP]`
- [ ] Port 9100 is open: `nc -zv [PRINTER_IP] 9100`
- [ ] Printer supports raw TCP/IP printing

### 10. **Still Not Working?** 🆘

1. **Get your printer's exact IP address**
2. **Test the connection manually:**
   ```bash
   nc -zv [PRINTER_IP] 9100
   ```
3. **If connection works, manually add via API** (see #6)
4. **Check server console output** for scanning messages
5. **Try different ports** (9100, 515, 631 for IPP)

---

## Summary: What You NEED

1. ✅ Printer IP address
2. ✅ Printer on same network as your computer  
3. ✅ Printer listening on port 9100 (or 515)
4. ✅ Network printing enabled on printer
5. ✅ If auto-detection fails → **Manually add via API**

The automatic scanner is a convenience feature, but **manual addition via API is the most reliable method**.
