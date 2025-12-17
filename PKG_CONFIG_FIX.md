# USB Support - Statically Linked

## Overview
USB printer support is **statically linked** into the binaries. This means USB printers work out of the box - no separate installation of libusb or any other dependencies required!

## How It Works
- **libusb is bundled**: The USB library is statically linked into the binary during build
- **Zero dependencies**: Users don't need to install anything - just download and run
- **Works everywhere**: USB printers are detected and work immediately

## For Developers Building from Source

If you're building from source, you'll need libusb development files during the build process:

### macOS
```bash
brew install libusb
```

### Linux
```bash
sudo apt-get install libusb-1.0-0-dev
```

### Windows
libusb is included with MSYS2/MinGW when building on Windows.

**Note**: These are only needed during build time. The resulting binaries include libusb statically linked and work without any dependencies.

## Benefits
- ✅ **Out of the box**: USB support works immediately from binaries
- ✅ **No installation hassle**: Users don't need to install libusb separately
- ✅ **Portable**: Binaries are self-contained
- ✅ **Reliable**: No dependency version conflicts
