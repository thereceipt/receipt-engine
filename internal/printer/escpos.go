package printer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
)

// ESC/POS commands
const (
	ESC byte = 0x1B
	GS  byte = 0x1D
	FS  byte = 0x1C
)

// ESCPOSEncoder generates ESC/POS commands from images
type ESCPOSEncoder struct {
	buffer *bytes.Buffer
}

// NewESCPOSEncoder creates a new ESC/POS encoder
func NewESCPOSEncoder() *ESCPOSEncoder {
	return &ESCPOSEncoder{
		buffer: new(bytes.Buffer),
	}
}

// Initialize sends initialization command
func (e *ESCPOSEncoder) Initialize() {
	e.buffer.WriteByte(ESC)
	e.buffer.WriteByte('@')
}

// PrintImage converts an image to ESC/POS raster graphics
func (e *ESCPOSEncoder) PrintImage(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	// Convert to 1-bit bitmap
	bitmap := imageToBitmap(img)
	
	// Calculate bytes per line (must be multiple of 8 pixels)
	bytesPerLine := (width + 7) / 8
	
	// Print using raster bit image command
	for y := 0; y < height; y++ {
		// ESC * m nL nH d1...dk
		// m = mode (0: normal, 1: double width, 2: double height, 3: quadruple)
		// nL, nH = number of bytes in horizontal direction
		
		e.buffer.WriteByte(ESC)
		e.buffer.WriteByte('*')
		e.buffer.WriteByte(33) // 24-dot double-density mode
		e.buffer.WriteByte(byte(bytesPerLine & 0xFF))
		e.buffer.WriteByte(byte((bytesPerLine >> 8) & 0xFF))
		
		// Write line data
		lineData := bitmap[y*bytesPerLine : (y+1)*bytesPerLine]
		e.buffer.Write(lineData)
		
		// Line feed
		e.LineFeed()
	}
	
	return nil
}

// Cut sends paper cut command
func (e *ESCPOSEncoder) Cut() {
	// Full cut
	e.buffer.WriteByte(GS)
	e.buffer.WriteByte('V')
	e.buffer.WriteByte(0)
}

// PartialCut sends partial cut command
func (e *ESCPOSEncoder) PartialCut() {
	e.buffer.WriteByte(GS)
	e.buffer.WriteByte('V')
	e.buffer.WriteByte(1)
}

// LineFeed sends line feed
func (e *ESCPOSEncoder) LineFeed() {
	e.buffer.WriteByte(0x0A)
}

// Feed sends multiple line feeds
func (e *ESCPOSEncoder) Feed(lines int) {
	for i := 0; i < lines; i++ {
		e.LineFeed()
	}
}

// SetAlignment sets text alignment
func (e *ESCPOSEncoder) SetAlignment(align string) {
	e.buffer.WriteByte(ESC)
	e.buffer.WriteByte('a')
	
	switch align {
	case "left":
		e.buffer.WriteByte(0)
	case "center":
		e.buffer.WriteByte(1)
	case "right":
		e.buffer.WriteByte(2)
	default:
		e.buffer.WriteByte(0)
	}
}

// SetTextSize sets text size
func (e *ESCPOSEncoder) SetTextSize(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if width > 8 {
		width = 8
	}
	if height > 8 {
		height = 8
	}
	
	size := byte(((width-1)<<4) | (height - 1))
	
	e.buffer.WriteByte(GS)
	e.buffer.WriteByte('!')
	e.buffer.WriteByte(size)
}

// SetBold enables or disables bold text
func (e *ESCPOSEncoder) SetBold(enabled bool) {
	e.buffer.WriteByte(ESC)
	e.buffer.WriteByte('E')
	if enabled {
		e.buffer.WriteByte(1)
	} else {
		e.buffer.WriteByte(0)
	}
}

// WriteText writes text
func (e *ESCPOSEncoder) WriteText(text string) {
	e.buffer.WriteString(text)
}

// GetBytes returns the generated ESC/POS commands
func (e *ESCPOSEncoder) GetBytes() []byte {
	return e.buffer.Bytes()
}

// Reset clears the buffer
func (e *ESCPOSEncoder) Reset() {
	e.buffer.Reset()
}

// imageToBitmap converts an image to a 1-bit bitmap
func imageToBitmap(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	bytesPerLine := (width + 7) / 8
	bitmap := make([]byte, bytesPerLine*height)
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Get pixel
			oldPixel := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			r, g, b, _ := oldPixel.RGBA()
			
			// Convert to grayscale
			gray := (r + g + b) / 3
			
			// Threshold at 50% (32768 out of 65535)
			if gray < 32768 {
				// Black pixel - set bit
				byteIndex := y*bytesPerLine + x/8
				bitIndex := 7 - (x % 8)
				bitmap[byteIndex] |= 1 << bitIndex
			}
		}
	}
	
	return bitmap
}

// EncodeImageToESCPOS is a helper function to convert an image to ESC/POS
func EncodeImageToESCPOS(img image.Image) []byte {
	encoder := NewESCPOSEncoder()
	encoder.Initialize()
	encoder.PrintImage(img)
	encoder.Feed(3)
	encoder.Cut()
	return encoder.GetBytes()
}
