package utils

import (
	"github.com/skip2/go-qrcode"
)

// GenerateQRCode generates a QR code PNG as byte slice
func GenerateQRCode(content string, size int) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, size)
}
