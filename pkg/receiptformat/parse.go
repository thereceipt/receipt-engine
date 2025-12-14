package receiptformat

import (
	"encoding/json"
	"fmt"
	"os"
)

// Parse parses a .receipt file from a byte slice
func Parse(data []byte) (*Receipt, error) {
	// First, unmarshal into a temporary struct to capture legacy font field
	var temp struct {
		Version        string                    `json:"version"`
		Name           string                    `json:"name,omitempty"`
		Description    string                    `json:"description,omitempty"`
		CreatedWith    string                    `json:"created_with,omitempty"`
		PaperWidth     string                    `json:"paper_width,omitempty"`
		Font           string                    `json:"font,omitempty"` // Legacy field
		Fonts          map[string]FontFamily     `json:"fonts,omitempty"`
		Variables      []Variable                `json:"variables,omitempty"`
		VariableArrays []VariableArray           `json:"variableArrays,omitempty"`
		Commands       []Command                 `json:"commands"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, fmt.Errorf("failed to parse receipt: %w", err)
	}
	
	// Migrate legacy font to fonts.default if present
	receipt := Receipt{
		Version:        temp.Version,
		Name:           temp.Name,
		Description:    temp.Description,
		CreatedWith:    temp.CreatedWith,
		PaperWidth:     temp.PaperWidth,
		Fonts:          temp.Fonts,
		Variables:      temp.Variables,
		VariableArrays: temp.VariableArrays,
		Commands:       temp.Commands,
	}
	
	// If legacy font exists and fonts.default doesn't exist, migrate it
	if temp.Font != "" {
		if receipt.Fonts == nil {
			receipt.Fonts = make(map[string]FontFamily)
		}
		
		// Only migrate if fonts.default doesn't already exist
		if _, exists := receipt.Fonts["default"]; !exists {
			// Create a variable font family for the legacy font path
			receipt.Fonts["default"] = FontFamily{
				Type: "variable",
				Path: temp.Font,
			}
		}
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
