package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/fogleman/gg"
	"github.com/yourusername/receipt-engine/pkg/receiptformat"
)

func (r *Renderer) renderText(cmd *receiptformat.Command) error {
	text := cmd.Value
	size := float64(cmd.Size)
	if size == 0 {
		size = 24
	}
	
	weight := cmd.Weight
	if weight == "" {
		weight = "normal"
	}
	
	align := cmd.Align
	if align == "" {
		align = "left"
	}
	
	// Load font
	fontPath := r.getFontPath(cmd.FontFamily, weight, cmd.Italic)
	if err := r.ctx.LoadFontFace(fontPath, size); err != nil {
		// Fallback to default if font loading fails
		fmt.Printf("Warning: failed to load font %s, using default\n", fontPath)
	}
	
	// Measure text
	textWidth, textHeight := r.ctx.MeasureString(text)
	
	// Calculate X position based on alignment
	var x float64
	switch align {
	case "center":
		x = float64(r.width)/2 - textWidth/2
	case "right":
		x = float64(r.width) - textWidth - 5
	default: // left
		x = 5
	}
	
	// Ensure we have enough height
	r.ensureHeight(int(textHeight) + 20)
	
	// Draw text
	r.ctx.DrawString(text, x, r.y+textHeight)
	
	// Move Y position
	r.y += textHeight + 10
	
	return nil
}

func (r *Renderer) getFontPath(family, weight string, italic bool) string {
	// TODO: Implement proper font loading from receipt.fonts
	// For now, use system defaults
	
	// Try common system font paths
	fontPaths := []string{
		"/System/Library/Fonts/Helvetica.ttc",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"C:\\Windows\\Fonts\\arial.ttf",
	}
	
	for _, path := range fontPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	// Last resort: use gg default
	return ""
}

func (r *Renderer) renderFeed(cmd *receiptformat.Command) error {
	lines := cmd.Lines
	if lines == 0 {
		lines = 1
	}
	
	lineHeight := 20.0
	r.y += float64(lines) * lineHeight
	
	return nil
}

func (r *Renderer) renderCut(cmd *receiptformat.Command) error {
	// Draw scissors icon or cut line
	r.ensureHeight(40)
	
	// Draw dashed line to indicate cut
	r.ctx.SetLineWidth(1)
	dashLength := 5.0
	x := 10.0
	y := r.y + 20
	
	for x < float64(r.width)-10 {
		r.ctx.DrawLine(x, y, x+dashLength, y)
		r.ctx.Stroke()
		x += dashLength * 2
	}
	
	r.y += 40
	
	return nil
}
