package renderer

import (
	"encoding/base64"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	
	"github.com/disintegration/imaging"
	"github.com/yourusername/receipt-engine/pkg/receiptformat"
)

func (r *Renderer) renderImage(cmd *receiptformat.Command) error {
	var img image.Image
	var err error
	
	if cmd.Base64 != "" {
		// Decode base64
		data, err := base64.StdEncoding.DecodeString(cmd.Base64)
		if err != nil {
			return err
		}
		
		img, _, err = image.Decode(strings.NewReader(string(data)))
		if err != nil {
			return err
		}
	} else if cmd.Path != "" {
		// Load from file
		file, err := os.Open(cmd.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		
		img, _, err = image.Decode(file)
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	
	// Resize to fit printer width
	if img.Bounds().Dx() != r.width {
		img = imaging.Resize(img, r.width, 0, imaging.Lanczos)
	}
	
	// Convert to 1-bit (black & white) with threshold
	threshold := cmd.Threshold
	if threshold == 0 {
		threshold = 128
	}
	
	bwImg := convertToBlackWhite(img, uint8(threshold))
	
	// Ensure we have enough height
	imgHeight := bwImg.Bounds().Dy()
	r.ensureHeight(imgHeight)
	
	// Draw image
	r.ctx.DrawImage(bwImg, 0, int(r.y))
	
	r.y += float64(imgHeight)
	
	return nil
}

func convertToBlackWhite(img image.Image, threshold uint8) image.Image {
	bounds := img.Bounds()
	bw := image.NewGray(bounds)
	
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			oldPixel := img.At(x, y)
			r, g, b, _ := oldPixel.RGBA()
			
			// Convert to grayscale
			gray := uint8((r + g + b) / 3 / 256)
			
			// Apply threshold
			var newGray uint8
			if gray < threshold {
				newGray = 0 // Black
			} else {
				newGray = 255 // White
			}
			
			bw.SetGray(x, y, color.Gray{Y: newGray})
		}
	}
	
	return bw
}
