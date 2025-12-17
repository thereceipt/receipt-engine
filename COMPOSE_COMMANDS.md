# --compose Command Reference

This document lists all commands available for the `--compose` feature.

## Basic Syntax

```bash
print <printer-id> --compose <command1> [property1:value1] [property2:value2] <command2> ...
```

Each command starts with a command type (e.g., `text:`, `feed:`, `cut`), followed by optional properties.

## Available Commands

### 1. Text Command
**Syntax:** `text:"Your text here" [properties]`

**Properties:**
- `size:<number>` - Font size in points (e.g., `size:24`)
- `align:left|center|right` - Text alignment
- `weight:normal|bold` - Font weight
- `italic:true|false` - Italic text
- `font_family:<name>` - Font family name

**Examples:**
```bash
text:"Hello World"
text:"Title" size:32 align:center weight:bold
text:"Subtitle" size:20 align:left
```

### 2. Feed Command
**Syntax:** `feed:<lines>`

**Properties:**
- `lines:<number>` - Number of lines to feed (specified in the command itself)

**Examples:**
```bash
feed:1
feed:3
```

### 3. Cut Command
**Syntax:** `cut`

No properties needed.

**Examples:**
```bash
cut
```

### 4. Divider Command
**Syntax:** `divider [properties]`

**Properties:**
- `style:solid|double|dashed|dotted` - Divider style
- `char:<character>` - Custom character for divider
- `length:<number>` - Divider length

**Examples:**
```bash
divider
divider style:solid
divider style:dashed
divider style:dotted char:"-"
```

### 6. Image Command
**Syntax:** `image:"/path/to/image.png" [properties]`

**Properties:**
- `path:<path>` - Image file path (specified in the command itself)
- `threshold:<number>` - Black/white threshold (0-255)
- `base64:<base64-string>` - Base64 encoded image (not supported in compose syntax)

**Examples:**
```bash
image:"/path/to/logo.png"
image:"logo.png" threshold:128
```

### 7. Barcode Command
**Syntax:** `barcode:"<value>" [properties]`

**Properties:**
- `value:<string>` - Barcode value (specified in the command itself)
- `format:CODE128|CODE39|EAN13|EAN8|UPC_A|UPC_E|ITF` - Barcode format
- `height:<number>` - Barcode height in pixels
- `width:<number>` - Barcode width multiplier
- `position:below|above|none` - Text position

**Examples:**
```bash
barcode:"1234567890"
barcode:"1234567890" format:CODE128 height:50
barcode:"1234567890" format:EAN13 height:60 position:below
```

### 8. QR Code Command
**Syntax:** `qrcode:"<value>" [properties]`

**Properties:**
- `value:<string>` - QR code data (specified in the command itself)
- `error_correction:L|M|Q|H` - Error correction level (Low, Medium, Quartile, High)

**Examples:**
```bash
qrcode:"https://example.com"
qrcode:"https://example.com" error_correction:M
qrcode:"Hello World" error_correction:H
```

## Complete Examples

### Simple Receipt
```bash
print printer-123 --compose text:"Hello World" size:32 align:center feed:2 cut
```

### Formatted Receipt
```bash
print printer-123 --compose \
  text:"STORE NAME" size:36 align:center weight:bold \
  feed:1 \
  divider style:solid \
  text:"Item 1" size:24 \
  text:"$10.00" size:24 align:right \
  feed:2 \
  cut
```

### Receipt with QR Code
```bash
print printer-123 --compose \
  text:"Thank you!" size:28 align:center \
  feed:1 \
  qrcode:"https://example.com/receipt/123" \
  feed:2 \
  cut
```

### Receipt with Barcode
```bash
print printer-123 --compose \
  text:"Order #12345" size:24 \
  feed:1 \
  barcode:"12345" format:CODE128 height:50 \
  feed:2 \
  cut
```

## Notes

- **Quotes**: Text values with spaces should be quoted: `text:"Hello World"`
- **Properties**: Properties come after the command: `text:"Hello" size:32 align:center`
- **Multiple Commands**: Separate commands with spaces
- **Alignment**: Use `align` as a property of text commands (e.g., `text:"Hello" align:center`), not as a standalone command
- **Not Supported**: Complex commands like `item`, `box`, and `folder` with nested commands are not supported in compose syntax (use a `.receipt` file for those)
