package renderer

import (
	
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/code39"
	"github.com/boombuler/barcode/ean"
	"github.com/skip2/go-qrcode"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

func (r *Renderer) renderBarcode(cmd *receiptformat.Command) error {
	if cmd.Value == "" {
		return nil
	}
	
	format := cmd.Format
	if format == "" {
		format = "CODE128"
	}
	
	height := cmd.Height
	if height == 0 {
		height = 80
	}
	
	width := cmd.Width
	if width == 0 {
		width = 2
	}
	
	var barcodeImg barcode.Barcode
	var err error
	
	// Generate barcode based on format
	switch format {
	case "CODE128":
		barcodeImg, err = code128.Encode(cmd.Value)
	case "CODE39":
		barcodeImg, err = code39.Encode(cmd.Value, false, false)
	case "EAN13":
		barcodeImg, err = ean.Encode(cmd.Value)
	case "EAN8":
		barcodeImg, err = ean.Encode(cmd.Value)
	default:
		barcodeImg, err = code128.Encode(cmd.Value)
	}
	
	if err != nil {
		return err
	}
	
	// Scale barcode
	targetWidth := r.width - 40 // Leave margins
	barcodeImg, err = barcode.Scale(barcodeImg, targetWidth, height)
	if err != nil {
		return err
	}
	
	// Ensure we have enough height
	imgHeight := barcodeImg.Bounds().Dy()
	r.ensureHeight(imgHeight + 20)
	
	// Center the barcode
	x := (r.width - barcodeImg.Bounds().Dx()) / 2
	
	// Draw barcode
	r.ctx.DrawImage(barcodeImg, x, int(r.y))
	
	r.y += float64(imgHeight) + 10
	
	return nil
}

func (r *Renderer) renderQRCode(cmd *receiptformat.Command) error {
	if cmd.Value == "" {
		return nil
	}
	
	size := cmd.Size
	if size == 0 {
		size = 6
	}
	
	errorCorrection := qrcode.Medium
	switch cmd.ErrorCorrection {
	case "L":
		errorCorrection = qrcode.Low
	case "M":
		errorCorrection = qrcode.Medium
	case "Q":
		errorCorrection = qrcode.High
	case "H":
		errorCorrection = qrcode.Highest
	}
	
	// Generate QR code
	qr, err := qrcode.New(cmd.Value, errorCorrection)
	if err != nil {
		return err
	}
	
	// Calculate size (make it fit printer width with margins)
	qrSize := r.width - 100 // Leave margins
	if qrSize > 400 {
		qrSize = 400 // Max size
	}
	
	qrImg := qr.Image(qrSize)
	
	// Ensure we have enough height
	imgHeight := qrImg.Bounds().Dy()
	r.ensureHeight(imgHeight + 20)
	
	// Center the QR code
	x := (r.width - qrImg.Bounds().Dx()) / 2
	
	// Draw QR code
	r.ctx.DrawImage(qrImg, x, int(r.y))
	
	r.y += float64(imgHeight) + 10
	
	return nil
}
