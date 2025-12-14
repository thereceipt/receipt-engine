package receiptformat

import (
	"testing"
)

func TestValidate_ValidReceipt(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Name:    "Test Receipt",
		Commands: []Command{
			{Type: "text", Value: "Hello World"},
			{Type: "cut"},
		},
	}
	
	if err := Validate(receipt); err != nil {
		t.Errorf("Expected valid receipt, got error: %v", err)
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	receipt := &Receipt{
		Commands: []Command{
			{Type: "text", Value: "Hello"},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for missing version")
	}
}

func TestValidate_InvalidVersion(t *testing.T) {
	receipt := &Receipt{
		Version: "2.0",
		Commands: []Command{
			{Type: "text", Value: "Hello"},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for invalid version")
	}
}

func TestValidate_NoCommands(t *testing.T) {
	receipt := &Receipt{
		Version:  "1.0",
		Commands: []Command{},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for no commands")
	}
}

func TestValidate_InvalidPaperWidth(t *testing.T) {
	receipt := &Receipt{
		Version:    "1.0",
		PaperWidth: "100mm",
		Commands: []Command{
			{Type: "text", Value: "Hello"},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for invalid paper width")
	}
}

func TestValidate_ValidPaperWidths(t *testing.T) {
	validWidths := []string{"58mm", "80mm", "112mm"}
	
	for _, width := range validWidths {
		receipt := &Receipt{
			Version:    "1.0",
			PaperWidth: width,
			Commands: []Command{
				{Type: "text", Value: "Hello"},
			},
		}
		
		if err := Validate(receipt); err != nil {
			t.Errorf("Expected valid for width %s, got error: %v", width, err)
		}
	}
}

func TestValidate_Variables(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Variables: []Variable{
			{Let: "storeName", ValueType: "string", DefaultValue: "My Store"},
			{Let: "total", ValueType: "double", DefaultValue: 10.50},
		},
		Commands: []Command{
			{Type: "text", DynamicValue: "storeName"},
			{Type: "text", DynamicValue: "total"},
		},
	}
	
	if err := Validate(receipt); err != nil {
		t.Errorf("Expected valid receipt with variables, got error: %v", err)
	}
}

func TestValidate_DuplicateVariableName(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Variables: []Variable{
			{Let: "name", ValueType: "string"},
			{Let: "name", ValueType: "string"},
		},
		Commands: []Command{
			{Type: "text", Value: "Hello"},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for duplicate variable name")
	}
}

func TestValidate_UnknownVariable(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Commands: []Command{
			{Type: "text", DynamicValue: "unknownVar"},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for unknown variable")
	}
}

func TestValidate_VariableArray(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		VariableArrays: []VariableArray{
			{
				Name: "products",
				Schema: []VariableArrayField{
					{Field: "name", ValueType: "string"},
					{Field: "price", ValueType: "double"},
				},
			},
		},
		Commands: []Command{
			{
				Type:         "item",
				ArrayBinding: "products",
				LeftSide:     []Command{{Type: "text", ArrayField: "name"}},
				RightSide:    []Command{{Type: "text", ArrayField: "price"}},
			},
		},
	}
	
	if err := Validate(receipt); err != nil {
		t.Errorf("Expected valid receipt with variable array, got error: %v", err)
	}
}

func TestValidate_UnknownArray(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Commands: []Command{
			{
				Type:         "item",
				ArrayBinding: "unknownArray",
				LeftSide:     []Command{{Type: "text", Value: "Left"}},
				RightSide:    []Command{{Type: "text", Value: "Right"}},
			},
		},
	}
	
	err := Validate(receipt)
	if err == nil {
		t.Error("Expected error for unknown array")
	}
}

func TestValidate_TextCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		wantErr bool
	}{
		{"valid static text", Command{Type: "text", Value: "Hello"}, false},
		{"valid align left", Command{Type: "text", Value: "Hello", Align: "left"}, false},
		{"valid align center", Command{Type: "text", Value: "Hello", Align: "center"}, false},
		{"valid align right", Command{Type: "text", Value: "Hello", Align: "right"}, false},
		{"invalid align", Command{Type: "text", Value: "Hello", Align: "invalid"}, true},
		{"no value", Command{Type: "text"}, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := &Receipt{
				Version:  "1.0",
				Commands: []Command{tt.cmd},
			}
			
			err := Validate(receipt)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_ImageCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmd     Command
		wantErr bool
	}{
		{"valid with path", Command{Type: "image", Path: "/path/to/image.png"}, false},
		{"valid with base64", Command{Type: "image", Base64: "base64data"}, false},
		{"invalid - no path or base64", Command{Type: "image"}, true},
		{"invalid - both path and base64", Command{Type: "image", Path: "/path", Base64: "data"}, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := &Receipt{
				Version:  "1.0",
				Commands: []Command{tt.cmd},
			}
			
			err := Validate(receipt)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_BarcodeCommand(t *testing.T) {
	validReceipt := &Receipt{
		Version: "1.0",
		Commands: []Command{
			{Type: "barcode", Value: "123456", Format: "CODE128"},
		},
	}
	
	if err := Validate(validReceipt); err != nil {
		t.Errorf("Expected valid barcode, got error: %v", err)
	}
	
	invalidReceipt := &Receipt{
		Version: "1.0",
		Commands: []Command{
			{Type: "barcode", Format: "CODE128"}, // Missing value
		},
	}
	
	if err := Validate(invalidReceipt); err == nil {
		t.Error("Expected error for barcode without value")
	}
}

func TestParse_ValidJSON(t *testing.T) {
	jsonData := `{
		"version": "1.0",
		"name": "Test Receipt",
		"commands": [
			{"type": "text", "value": "Hello World"},
			{"type": "cut"}
		]
	}`
	
	receipt, err := Parse([]byte(jsonData))
	if err != nil {
		t.Errorf("Expected successful parse, got error: %v", err)
	}
	
	if receipt.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", receipt.Version)
	}
	if receipt.Name != "Test Receipt" {
		t.Errorf("Expected name 'Test Receipt', got %s", receipt.Name)
	}
	if len(receipt.Commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(receipt.Commands))
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	jsonData := `{invalid json`
	
	_, err := Parse([]byte(jsonData))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestToJSON(t *testing.T) {
	receipt := &Receipt{
		Version: "1.0",
		Name:    "Test",
		Commands: []Command{
			{Type: "text", Value: "Hello"},
		},
	}
	
	jsonData, err := receipt.ToJSON()
	if err != nil {
		t.Errorf("Expected successful JSON conversion, got error: %v", err)
	}
	
	// Parse it back
	parsed, err := Parse(jsonData)
	if err != nil {
		t.Errorf("Expected successful re-parse, got error: %v", err)
	}
	
	if parsed.Name != receipt.Name {
		t.Errorf("Round-trip failed: expected name %s, got %s", receipt.Name, parsed.Name)
	}
}
