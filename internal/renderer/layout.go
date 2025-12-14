package renderer

import (
	"github.com/fogleman/gg"
	"github.com/yourusername/receipt-engine/pkg/receiptformat"
	"image"
	"image/color"
)

func (r *Renderer) renderItem(cmd *receiptformat.Command) error {
	// Parse width ratio
	leftRatio, rightRatio := 1, 1
	if cmd.WidthRatio != "" {
		fmt.Sscanf(cmd.WidthRatio, "%d:%d", &leftRatio, &rightRatio)
	}
	totalRatio := leftRatio + rightRatio
	
	// Calculate widths
	dividerWidth := 0
	if cmd.ShowDivider {
		dividerWidth = 2
	}
	
	availableWidth := r.width - dividerWidth
	leftWidth := (availableWidth * leftRatio) / totalRatio
	rightWidth := availableWidth - leftWidth
	
	// Render left side to its own context
	leftCtx := gg.NewContext(leftWidth, 1000)
	leftCtx.SetColor(color.White)
	leftCtx.Clear()
	leftCtx.SetColor(color.Black)
	
	leftRenderer := &Renderer{
		width:  leftWidth,
		height: 1000,
		ctx:    leftCtx,
		y:      0,
	}
	
	for _, subCmd := range cmd.LeftSide {
		leftRenderer.renderCommand(&subCmd)
	}
	
	leftImg := leftRenderer.cropToContent()
	
	// Render right side
	rightCtx := gg.NewContext(rightWidth, 1000)
	rightCtx.SetColor(color.White)
	rightCtx.Clear()
	rightCtx.SetColor(color.Black)
	
	rightRenderer := &Renderer{
		width:  rightWidth,
		height: 1000,
		ctx:    rightCtx,
		y:      0,
	}
	
	for _, subCmd := range cmd.RightSide {
		rightRenderer.renderCommand(&subCmd)
	}
	
	rightImg := rightRenderer.cropToContent()
	
	// Determine combined height
	combinedHeight := leftImg.Bounds().Dy()
	if rightImg.Bounds().Dy() > combinedHeight {
		combinedHeight = rightImg.Bounds().Dy()
	}
	
	// Ensure we have enough height
	r.ensureHeight(combinedHeight)
	
	// Draw left side
	r.ctx.DrawImage(leftImg, 0, int(r.y))
	
	// Draw divider if needed
	if cmd.ShowDivider {
		dividerX := float64(leftWidth + 1)
		dividerStyle := cmd.DividerStyle
		if dividerStyle == "" {
			dividerStyle = "solid"
		}
		
		r.ctx.SetLineWidth(float64(dividerWidth))
		
		switch dividerStyle {
		case "solid":
			r.ctx.DrawLine(dividerX, r.y, dividerX, r.y+float64(combinedHeight))
			r.ctx.Stroke()
		case "dashed":
			dashLen := 8.0
			gapLen := 4.0
			y := r.y
			for y < r.y+float64(combinedHeight) {
				endY := y + dashLen
				if endY > r.y+float64(combinedHeight) {
					endY = r.y + float64(combinedHeight)
				}
				r.ctx.DrawLine(dividerX, y, dividerX, endY)
				r.ctx.Stroke()
				y += dashLen + gapLen
			}
		case "dotted":
			dotSpacing := 6.0
			y := r.y
			for y < r.y+float64(combinedHeight) {
				r.ctx.DrawCircle(dividerX, y, 1)
				r.ctx.Fill()
				y += dotSpacing
			}
		}
	}
	
	// Draw right side
	rightX := leftWidth + dividerWidth
	r.ctx.DrawImage(rightImg, rightX, int(r.y))
	
	r.y += float64(combinedHeight)
	
	return nil
}

func (r *Renderer) renderBox(cmd *receiptformat.Command) error {
	// Get box properties
	width := r.width
	if cmd.Width != "" && cmd.Width != "full" {
		// Parse percentage or pixel value
		if cmd.Width[len(cmd.Width)-1] == '%' {
			var pct int
			fmt.Sscanf(cmd.Width, "%d%%", &pct)
			width = (r.width * pct) / 100
		}
	}
	
	border := cmd.Border
	if border == 0 {
		border = 2
	}
	
	padding := cmd.Padding
	if padding == 0 {
		padding = 10
	}
	
	margin := cmd.Margin
	
	// Render box contents to temporary context
	contentWidth := width - 2*border - 2*padding
	contentCtx := gg.NewContext(contentWidth, 2000)
	contentCtx.SetColor(color.White)
	contentCtx.Clear()
	contentCtx.SetColor(color.Black)
	
	contentRenderer := &Renderer{
		width:  contentWidth,
		height: 2000,
		ctx:    contentCtx,
		y:      0,
	}
	
	// Render title if present
	titleHeight := 0
	if cmd.Title != "" {
		titleCmd := receiptformat.Command{
			Type:   "text",
			Value:  cmd.Title,
			Size:   18,
			Weight: "bold",
			Align:  "center",
		}
		contentRenderer.renderCommand(&titleCmd)
		titleHeight = int(contentRenderer.y) + 5
	}
	
	// Render nested commands
	for _, subCmd := range cmd.Commands {
		contentRenderer.renderCommand(&subCmd)
	}
	
	contentImg := contentRenderer.cropToContent()
	contentHeight := contentImg.Bounds().Dy()
	
	// Calculate box height
	boxHeight := contentHeight + 2*border + 2*padding
	
	// Ensure we have enough height
	r.ensureHeight(boxHeight + 2*margin)
	
	// Calculate X position based on alignment
	var boxX int
	align := cmd.Align
	if align == "" {
		align = "center"
	}
	
	switch align {
	case "center":
		boxX = (r.width - width) / 2
	case "right":
		boxX = r.width - width - margin
	default: // left
		boxX = margin
	}
	
	boxY := int(r.y) + margin
	
	// Create box background
	if cmd.Inverted {
		r.ctx.SetColor(color.Black)
	} else {
		r.ctx.SetColor(color.White)
	}
	
	if cmd.BorderRadius > 0 {
		r.ctx.DrawRoundedRectangle(float64(boxX), float64(boxY), float64(width), float64(boxHeight), float64(cmd.BorderRadius))
		r.ctx.Fill()
	} else {
		r.ctx.DrawRectangle(float64(boxX), float64(boxY), float64(width), float64(boxHeight))
		r.ctx.Fill()
	}
	
	// Draw border
	if border > 0 {
		if cmd.Inverted {
			r.ctx.SetColor(color.White)
		} else {
			r.ctx.SetColor(color.Black)
		}
		r.ctx.SetLineWidth(float64(border))
		
		if cmd.BorderRadius > 0 {
			r.ctx.DrawRoundedRectangle(float64(boxX), float64(boxY), float64(width), float64(boxHeight), float64(cmd.BorderRadius))
		} else {
			r.ctx.DrawRectangle(float64(boxX), float64(boxY), float64(width), float64(boxHeight))
		}
		r.ctx.Stroke()
	}
	
	// Draw content (inverted if needed)
	contentX := boxX + border + padding
	contentY := boxY + border + padding
	
	if cmd.Inverted {
		// Invert content colors
		invertedContent := invertImage(contentImg)
		r.ctx.DrawImage(invertedContent, contentX, contentY)
	} else {
		r.ctx.DrawImage(contentImg, contentX, contentY)
	}
	
	r.y += float64(boxHeight + 2*margin)
	
	return nil
}

func invertImage(img image.Image) image.Image {
	bounds := img.Bounds()
	inverted := image.NewRGBA(bounds)
	
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			oldPixel := img.At(x, y)
			r, g, b, a := oldPixel.RGBA()
			
			// Invert RGB but keep alpha
			inverted.Set(x, y, color.RGBA{
				R: uint8(255 - r/256),
				G: uint8(255 - g/256),
				B: uint8(255 - b/256),
				A: uint8(a / 256),
			})
		}
	}
	
	return inverted
}
