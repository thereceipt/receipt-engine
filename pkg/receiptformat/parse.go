package receiptformat

import (
	"encoding/json"
	"fmt"
	"os"
)

// Parse parses a .receipt file from a byte slice
func Parse(data []byte) (*Receipt, error) {
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}
	
	if err := Validate(&receipt); err != nil {
		return nil, err
	}
	
	return &receipt, nil
}

// ParseFile parses a .receipt file from disk
func ParseFile(path string) (*Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read receipt file: %w", err)
	}
	
	return Parse(data)
}

// ToJSON converts a Receipt to JSON bytes
func (r *Receipt) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// SaveToFile saves a Receipt to a file
func (r *Receipt) SaveToFile(path string) error {
	data, err := r.ToJSON()
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}
