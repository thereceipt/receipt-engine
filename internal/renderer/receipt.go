package renderer

import (
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// SetReceipt sets the receipt reference for font loading
func (r *Renderer) SetReceipt(receipt *receiptformat.Receipt) {
	r.receipt = receipt
}
