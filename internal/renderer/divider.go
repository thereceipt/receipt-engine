package renderer

import (
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

func (r *Renderer) renderDivider(cmd *receiptformat.Command) error {
	style := cmd.Style
	if style == "" {
		style = "solid"
	}
	
	r.ensureHeight(15)
	
	y := r.y + 7
	margin := 20.0
	x1 := margin
	x2 := float64(r.width) - margin
	
	r.ctx.SetLineWidth(2)
	
	switch style {
	case "solid":
		r.ctx.DrawLine(x1, y, x2, y)
		r.ctx.Stroke()
		
	case "double":
		r.ctx.DrawLine(x1, y-2, x2, y-2)
		r.ctx.Stroke()
		r.ctx.DrawLine(x1, y+2, x2, y+2)
		r.ctx.Stroke()
		
	case "dashed":
		dashLength := 10.0
		gapLength := 5.0
		x := x1
		for x < x2 {
			endX := x + dashLength
			if endX > x2 {
				endX = x2
			}
			r.ctx.DrawLine(x, y, endX, y)
			r.ctx.Stroke()
			x += dashLength + gapLength
		}
		
	case "dotted":
		dotSpacing := 8.0
		x := x1
		for x < x2 {
			r.ctx.DrawCircle(x, y, 1)
			r.ctx.Fill()
			x += dotSpacing
		}
	}
	
	r.y += 15
	
	return nil
}
