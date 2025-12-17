package renderer

import (
	"os"

	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

func (r *Renderer) renderText(cmd *receiptformat.Command) error {
	text := cmd.Value

	// Get size - handle both int and potential float64 from JSON
	size := float64(cmd.Size)
	if size == 0 {
		size = 32 // Default size (increased from 24)
	}

	// Debug: log the size being used
	if cmd.Size != 0 {
		// Debug: Using font size (removed to avoid TUI interference)
	} else {
		// Debug: Using default font size (removed to avoid TUI interference)
	}

	weight := cmd.Weight
	if weight == "" {
		weight = "normal"
	}

	align := cmd.Align
	if align == "" {
		align = "left"
	}

	// Load font with the specified size
	// If no font_family is specified, use "default"
	fontFamily := cmd.FontFamily
	if fontFamily == "" {
		fontFamily = "default"
	}
	fontPath := r.getFontPath(fontFamily, weight, cmd.Italic)

	// Always try to load a font with the specified size
	// If the preferred font fails, fall back to system fonts
	loaded := false
	if fontPath != "" {
		if err := r.ctx.LoadFontFace(fontPath, size); err == nil {
			loaded = true
		} else {
			// Warning: failed to load font - will fall back to system font
		}
	}

	// If preferred font didn't load, try system fonts
	if !loaded {
		systemFonts := []string{
			"/System/Library/Fonts/Helvetica.ttc",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		}
		for _, font := range systemFonts {
			if _, err := os.Stat(font); err == nil {
				if err := r.ctx.LoadFontFace(font, size); err == nil {
					loaded = true
					break
				}
			}
		}
		if !loaded {
			// Warning: could not load any font - using system default
		}
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
	// Normalize weight - convert "normal" to "regular" for matching
	if weight == "normal" {
		weight = "regular"
	}

	// Check if receipt has custom fonts defined
	if r.receipt != nil && len(r.receipt.Fonts) > 0 {
		// Fonts is a map[string]FontFamily, key is the family name
		if fontFamily, exists := r.receipt.Fonts[family]; exists {
			// For static fonts, check the weights array
			if fontFamily.Type == "static" && len(fontFamily.Weights) > 0 {
				for _, fw := range fontFamily.Weights {
					matchesWeight := weight == "" || fw.Weight == weight
					matchesItalic := fw.Italic == italic

					if matchesWeight && matchesItalic {
						return fw.Path
					}
				}
			}

			// For variable fonts, or if no matching weight found, use the main path
			if fontFamily.Path != "" {
				return fontFamily.Path
			}
		}
	}

	// Fallback to system fonts
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
	// Just add some space before the cut - the actual cut is handled by the printer
	// No visual divider needed
	r.ensureHeight(20)
	r.y += 20

	return nil
}
