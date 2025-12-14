package printer

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	
	"github.com/hennedo/escpos"
)

// EncodeImageToESCPOS converts an image to ESC/POS commands using the escpos library
// This matches the Python escpos library approach
func EncodeImageToESCPOS(img image.Image) []byte {
	var buf bytes.Buffer
	
	// Convert image to RGBA format with (0,0) origin (library expects this)
	// The library's getPixels function assumes bounds start at (0,0)
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	// Pad dimensions to be multiples of 8 (library requirement)
	// This prevents out-of-bounds access and garbage data
	paddedWidth := ((width + 7) / 8) * 8
	paddedHeight := ((height + 7) / 8) * 8
	
	// Create RGBA image with padded dimensions (white background)
	rgbaImg := image.NewRGBA(image.Rect(0, 0, paddedWidth, paddedHeight))
	
	// Fill with white background
	white := color.RGBA{255, 255, 255, 255}
	for y := 0; y < paddedHeight; y++ {
		for x := 0; x < paddedWidth; x++ {
			rgbaImg.Set(x, y, white)
		}
	}
	
	// Draw the original image on top (at 0,0)
	draw.Draw(rgbaImg, image.Rect(0, 0, width, height), img, bounds.Min, draw.Src)
	
	// Create ESC/POS encoder (like Python's escpos library)
	e := escpos.New(&buf)
	
	// Initialize printer
	e.Initialize()
	
	// Print image (library handles all the ESC/POS encoding)
	e.PrintImage(rgbaImg)
	
	// Feed a few lines before cutting
	e.LineFeed()
	e.LineFeed()
	e.LineFeed()
	
	// Flush the buffered writer first
	e.Print()
	
	// Cut paper - library's Cut() has a bug, so write correct command manually
	// GS V 0 = Full cut (0x1D 0x56 0x00)
	buf.WriteByte(0x1D) // GS
	buf.WriteByte(0x56) // V
	buf.WriteByte(0x00) // 0 = Full cut
	
	return buf.Bytes()
}
