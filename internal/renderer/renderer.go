// Package renderer handles image rendering for receipts
package renderer

import (
	"fmt"
	"image"
	"image/color"
	
	"github.com/fogleman/gg"
	"github.com/yourusername/receipt-engine/pkg/receiptformat"
)

// Renderer converts receipt commands to images
type Renderer struct {
	width  int // Paper width in pixels
	height int // Current canvas height
	ctx    *gg.Context
	y      float64 // Current Y position
}

// New creates a new renderer
func New(paperWidth string) (*Renderer, error) {
	width := paperWidthToPixels(paperWidth)
	
	// Start with reasonable initial height, will grow as needed
	initialHeight := 1000
	
	ctx := gg.NewContext(width, initialHeight)
	ctx.SetColor(color.White)
	ctx.Clear()
	ctx.SetColor(color.Black)
	
	return &Renderer{
		width:  width,
		height: initialHeight,
		ctx:    ctx,
		y:      0,
	}, nil
}

// Render renders a complete receipt
func (r *Renderer) Render(receipt *receiptformat.Receipt) (image.Image, error) {
	for _, cmd := range receipt.Commands {
		if err := r.renderCommand(&cmd); err != nil {
			return nil, fmt.Errorf("failed to render command: %w", err)
		}
	}
	
	// Crop to actual content height
	return r.cropToContent(), nil
}

// GetImage returns the rendered image
func (r *Renderer) GetImage() image.Image {
	return r.ctx.Image()
}

func (r *Renderer) renderCommand(cmd *receiptformat.Command) error {
	switch cmd.Type {
	case "text":
		return r.renderText(cmd)
	case "feed":
		return r.renderFeed(cmd)
	case "cut":
		return r.renderCut(cmd)
	case "divider":
		return r.renderDivider(cmd)
	case "image":
		return r.renderImage(cmd)
	case "barcode":
		return r.renderBarcode(cmd)
	case "qrcode":
		return r.renderQRCode(cmd)
	case "item":
		return r.renderItem(cmd)
	case "box":
		return r.renderBox(cmd)
	default:
		return fmt.Errorf("unsupported command type: %s", cmd.Type)
	}
}

func (r *Renderer) cropToContent() image.Image {
	// Crop to the actual Y position used
	finalHeight := int(r.y) + 50 // Add small bottom margin
	if finalHeight > r.height {
		finalHeight = r.height
	}
	
	img := r.ctx.Image()
	return img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(image.Rect(0, 0, r.width, finalHeight))
}

func (r *Renderer) ensureHeight(neededHeight int) {
	if int(r.y)+neededHeight > r.height {
		// Need to expand canvas
		newHeight := r.height * 2
		if newHeight < int(r.y)+neededHeight {
			newHeight = int(r.y) + neededHeight + 1000
		}
		
		// Create new context with larger size
		newCtx := gg.NewContext(r.width, newHeight)
		newCtx.SetColor(color.White)
		newCtx.Clear()
		
		// Copy existing content
		newCtx.DrawImage(r.ctx.Image(), 0, 0)
		
		r.ctx = newCtx
		r.height = newHeight
	}
}

func paperWidthToPixels(width string) int {
	switch width {
	case "58mm":
		return 384
	case "80mm":
		return 576
	case "112mm":
		return 832
	default:
		return 576 // Default to 80mm
	}
}
