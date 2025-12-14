// Package parser handles command parsing and execution
package parser

import (
	"fmt"
	"image"
	
	"github.com/thereceipt/receipt-engine/internal/renderer"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// Parser executes receipt commands with variable and array support
type Parser struct {
	receipt           *receiptformat.Receipt
	renderer          *renderer.Renderer
	variableData      map[string]interface{}
	variableArrayData map[string][]map[string]interface{}
}

// New creates a new parser
func New(receipt *receiptformat.Receipt, paperWidth string) (*Parser, error) {
	r, err := renderer.New(paperWidth)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}
	
	// Set receipt reference for font loading
	r.SetReceipt(receipt)
	
	return &Parser{
		receipt:           receipt,
		renderer:          r,
		variableData:      make(map[string]interface{}),
		variableArrayData: make(map[string][]map[string]interface{}),
	}, nil
}

// SetVariableData sets the data for template variables
func (p *Parser) SetVariableData(data map[string]interface{}) {
	p.variableData = data
}

// SetVariableArrayData sets the data for variable arrays
func (p *Parser) SetVariableArrayData(data map[string][]map[string]interface{}) {
	p.variableArrayData = data
}

// Execute parses and renders the receipt
func (p *Parser) Execute() (image.Image, error) {
	// Process commands
	for _, cmd := range p.receipt.Commands {
		if err := p.executeCommand(&cmd); err != nil {
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}
	
	return p.renderer.GetImage(), nil
}

func (p *Parser) executeCommand(cmd *receiptformat.Command) error {
	// Check if command has array binding
	if cmd.ArrayBinding != "" {
		return p.executeArrayBoundCommand(cmd)
	}
	
	// Resolve variables in command
	resolvedCmd, err := p.resolveCommand(cmd)
	if err != nil {
		return err
	}
	
	// Execute through renderer
	return p.renderer.RenderCommand(resolvedCmd)
}

func (p *Parser) executeArrayBoundCommand(cmd *receiptformat.Command) error {
	arrayName := cmd.ArrayBinding
	
	// Get array schema
	var schema receiptformat.VariableArray
	found := false
	for _, arr := range p.receipt.VariableArrays {
		if arr.Name == arrayName {
			schema = arr
			found = true
			break
		}
	}
	
	if !found {
		return fmt.Errorf("unknown variable array: %s", arrayName)
	}
	
	// Get data for this array
	dataEntries := p.variableArrayData[arrayName]
	
	// If no data provided, use defaults for preview
	if len(dataEntries) == 0 {
		defaultEntry := make(map[string]interface{})
		for _, field := range schema.Schema {
			defaultEntry[field.Field] = field.DefaultValue
		}
		dataEntries = []map[string]interface{}{defaultEntry}
	}
	
	// Render command once for each data entry
	for _, entry := range dataEntries {
		expandedCmd := p.expandArrayFields(cmd, &schema, entry)
		
		// Resolve any remaining variables
		resolvedCmd, err := p.resolveCommand(expandedCmd)
		if err != nil {
			return err
		}
		
		// Render
		if err := p.renderer.RenderCommand(resolvedCmd); err != nil {
			return err
		}
	}
	
	return nil
}

func (p *Parser) expandArrayFields(cmd *receiptformat.Command, schema *receiptformat.VariableArray, data map[string]interface{}) *receiptformat.Command {
	// Deep copy command
	expanded := *cmd
	
	// Remove array binding from expanded command
	expanded.ArrayBinding = ""
	
	// Expand arrayField references
	if expanded.ArrayField != "" {
		// Find field in schema
		var fieldDef *receiptformat.VariableArrayField
		for i := range schema.Schema {
			if schema.Schema[i].Field == expanded.ArrayField {
				fieldDef = &schema.Schema[i]
				break
			}
		}
		
		if fieldDef != nil {
			// Get value from data or use default
			value := data[expanded.ArrayField]
			if value == nil {
				value = fieldDef.DefaultValue
			}
			
			// Format with prefix/suffix
			formatted := p.formatValue(value, fieldDef.Prefix, fieldDef.Suffix)
			
			// Set as value
			expanded.Value = formatted
			expanded.ArrayField = ""
		}
	}
	
	// Recursively expand nested commands (for item, box, etc.)
	if len(expanded.LeftSide) > 0 {
		for i := range expanded.LeftSide {
			expandedSub := p.expandArrayFields(&expanded.LeftSide[i], schema, data)
			expanded.LeftSide[i] = *expandedSub
		}
	}
	
	if len(expanded.RightSide) > 0 {
		for i := range expanded.RightSide {
			expandedSub := p.expandArrayFields(&expanded.RightSide[i], schema, data)
			expanded.RightSide[i] = *expandedSub
		}
	}
	
	if len(expanded.Commands) > 0 {
		for i := range expanded.Commands {
			expandedSub := p.expandArrayFields(&expanded.Commands[i], schema, data)
			expanded.Commands[i] = *expandedSub
		}
	}
	
	return &expanded
}

func (p *Parser) resolveCommand(cmd *receiptformat.Command) (*receiptformat.Command, error) {
	resolved := *cmd
	
	// Resolve dynamicValue
	if resolved.DynamicValue != "" {
		// Find variable definition
		var varDef *receiptformat.Variable
		for i := range p.receipt.Variables {
			if p.receipt.Variables[i].Let == resolved.DynamicValue {
				varDef = &p.receipt.Variables[i]
				break
			}
		}
		
		if varDef != nil {
			// Get value from data or use default
			value := p.variableData[resolved.DynamicValue]
			if value == nil {
				value = varDef.DefaultValue
			}
			
			// Format with prefix/suffix
			formatted := p.formatValue(value, varDef.Prefix, varDef.Suffix)
			
			// Set as value
			resolved.Value = formatted
			resolved.DynamicValue = ""
		}
	}
	
	// Recursively resolve nested commands
	if len(resolved.LeftSide) > 0 {
		for i := range resolved.LeftSide {
			resolvedSub, err := p.resolveCommand(&resolved.LeftSide[i])
			if err != nil {
				return nil, err
			}
			resolved.LeftSide[i] = *resolvedSub
		}
	}
	
	if len(resolved.RightSide) > 0 {
		for i := range resolved.RightSide {
			resolvedSub, err := p.resolveCommand(&resolved.RightSide[i])
			if err != nil {
				return nil, err
			}
			resolved.RightSide[i] = *resolvedSub
		}
	}
	
	if len(resolved.Commands) > 0 {
		for i := range resolved.Commands {
			resolvedSub, err := p.resolveCommand(&resolved.Commands[i])
			if err != nil {
				return nil, err
			}
			resolved.Commands[i] = *resolvedSub
		}
	}
	
	return &resolved, nil
}

func (p *Parser) formatValue(value interface{}, prefix string, suffix string) string {
	if value == nil {
		return ""
	}
	
	return fmt.Sprintf("%s%v%s", prefix, value, suffix)
}
