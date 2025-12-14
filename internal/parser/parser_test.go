package parser

import (
	"testing"
	
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

func TestParser_SimpleReceipt(t *testing.T) {
	receipt := &receiptformat.Receipt{
		Version: "1.0",
		Commands: []receiptformat.Command{
			{Type: "text", Value: "Hello World", Size: 24},
			{Type: "feed", Lines: 1},
			{Type: "cut"},
		},
	}
	
	parser, err := New(receipt, "80mm")
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	img, err := parser.Execute()
	if err != nil {
		t.Fatalf("Failed to execute: %v", err)
	}
	
	if img == nil {
		t.Error("Expected non-nil image")
	}
}

func TestParser_WithVariables(t *testing.T) {
	receipt := &receiptformat.Receipt{
		Version: "1.0",
		Variables: []receiptformat.Variable{
			{Let: "storeName", ValueType: "string", DefaultValue: "My Store"},
			{Let: "total", ValueType: "double", DefaultValue: 10.50, Prefix: "$"},
		},
		Commands: []receiptformat.Command{
			{Type: "text", DynamicValue: "storeName", Size: 32, Align: "center"},
			{Type: "text", DynamicValue: "total", Size: 24},
		},
	}
	
	parser, _ := New(receipt, "80mm")
	
	// Set custom values
	parser.SetVariableData(map[string]interface{}{
		"storeName": "Coffee Shop",
		"total":     25.99,
	})
	
	img, err := parser.Execute()
	if err != nil {
		t.Fatalf("Failed to execute with variables: %v", err)
	}
	
	if img == nil {
		t.Error("Expected non-nil image")
	}
}

func TestParser_WithArrays(t *testing.T) {
	receipt := &receiptformat.Receipt{
		Version: "1.0",
		VariableArrays: []receiptformat.VariableArray{
			{
				Name: "products",
				Schema: []receiptformat.VariableArrayField{
					{Field: "name", ValueType: "string", DefaultValue: "Product"},
					{Field: "price", ValueType: "double", DefaultValue: 0.00, Prefix: "$"},
				},
			},
		},
		Commands: []receiptformat.Command{
			{
				Type:         "item",
				ArrayBinding: "products",
				LeftSide: []receiptformat.Command{
					{Type: "text", ArrayField: "name", Size: 20},
				},
				RightSide: []receiptformat.Command{
					{Type: "text", ArrayField: "price", Size: 20, Align: "right"},
				},
			},
		},
	}
	
	parser, _ := New(receipt, "80mm")
	
	// Set array data
	parser.SetVariableArrayData(map[string][]map[string]interface{}{
		"products": {
			{"name": "Coffee", "price": 3.50},
			{"name": "Croissant", "price": 2.75},
			{"name": "Orange Juice", "price": 4.00},
		},
	})
	
	img, err := parser.Execute()
	if err != nil {
		t.Fatalf("Failed to execute with arrays: %v", err)
	}
	
	if img == nil {
		t.Error("Expected non-nil image")
	}
}

func TestFormatValue(t *testing.T) {
	parser := &Parser{}
	
	tests := []struct {
		value    interface{}
		prefix   string
		suffix   string
		expected string
	}{
		{10.50, "$", "", "$10.5"},
		{5, "", "x", "5x"},
		{"Test", "Prefix:", ":Suffix", "Prefix:Test:Suffix"},
		{nil, "$", "", ""},
	}
	
	for _, tt := range tests {
		result := parser.formatValue(tt.value, tt.prefix, tt.suffix)
		if result != tt.expected {
			t.Errorf("formatValue(%v, %q, %q) = %q, want %q",
				tt.value, tt.prefix, tt.suffix, result, tt.expected)
		}
	}
}
