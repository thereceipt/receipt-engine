package renderer

import (
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// RenderCommand is a public wrapper for rendering a single command
func (r *Renderer) RenderCommand(cmd *receiptformat.Command) error {
	return r.renderCommand(cmd)
}
