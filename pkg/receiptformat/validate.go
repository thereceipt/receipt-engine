package receiptformat

import (
	"fmt"
	"strings"
)

// Validate validates a Receipt structure
func Validate(r *Receipt) error {
	// Validate version
	if r.Version == "" {
		return fmt.Errorf("version is required")
	}
	if r.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s (expected 1.0)", r.Version)
	}
	
	// Validate paper width if specified
	if r.PaperWidth != "" {
		validWidths := []string{"58mm", "80mm", "112mm"}
		valid := false
		for _, w := range validWidths {
			if r.PaperWidth == w {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid paper_width: %s (must be 58mm, 80mm, or 112mm)", r.PaperWidth)
		}
	}
	
	// Validate variables
	variableNames := make(map[string]bool)
	for i, v := range r.Variables {
		if v.Let == "" {
			return fmt.Errorf("variable[%d]: 'let' is required", i)
		}
		if variableNames[v.Let] {
			return fmt.Errorf("variable[%d]: duplicate variable name '%s'", i, v.Let)
		}
		variableNames[v.Let] = true
		
		if err := validateValueType(v.ValueType); err != nil {
			return fmt.Errorf("variable[%d] '%s': %w", i, v.Let, err)
		}
	}
	
	// Validate variable arrays
	arrayNames := make(map[string]bool)
	for i, arr := range r.VariableArrays {
		if arr.Name == "" {
			return fmt.Errorf("variableArray[%d]: 'name' is required", i)
		}
		if arrayNames[arr.Name] {
			return fmt.Errorf("variableArray[%d]: duplicate array name '%s'", i, arr.Name)
		}
		arrayNames[arr.Name] = true
		
		// Validate schema fields
		fieldNames := make(map[string]bool)
		for j, field := range arr.Schema {
			if field.Field == "" {
				return fmt.Errorf("variableArray[%d] '%s' field[%d]: 'field' is required", i, arr.Name, j)
			}
			if fieldNames[field.Field] {
				return fmt.Errorf("variableArray[%d] '%s' field[%d]: duplicate field name '%s'", i, arr.Name, j, field.Field)
			}
			fieldNames[field.Field] = true
			
			if err := validateValueType(field.ValueType); err != nil {
				return fmt.Errorf("variableArray[%d] '%s' field[%d] '%s': %w", i, arr.Name, j, field.Field, err)
			}
		}
	}
	
	// Validate commands
	if len(r.Commands) == 0 {
		return fmt.Errorf("at least one command is required")
	}
	
	for i, cmd := range r.Commands {
		if err := validateCommand(&cmd, variableNames, arrayNames); err != nil {
			return fmt.Errorf("command[%d]: %w", i, err)
		}
	}
	
	return nil
}

func validateValueType(vt string) error {
	validTypes := []string{"string", "number", "double", "boolean"}
	for _, t := range validTypes {
		if vt == t {
			return nil
		}
	}
	return fmt.Errorf("invalid valueType '%s' (must be string, number, double, or boolean)", vt)
}

func validateCommand(cmd *Command, variables map[string]bool, arrays map[string]bool) error {
	if cmd.Type == "" {
		return fmt.Errorf("command type is required")
	}
	
	// Validate array binding if present
	if cmd.ArrayBinding != "" {
		if !arrays[cmd.ArrayBinding] {
			return fmt.Errorf("unknown array '%s' in arrayBinding", cmd.ArrayBinding)
		}
	}
	
	// Type-specific validation
	switch cmd.Type {
	case "text":
		return validateTextCommand(cmd, variables)
	case "item":
		return validateItemCommand(cmd, variables, arrays)
	case "image":
		return validateImageCommand(cmd)
	case "barcode":
		return validateBarcodeCommand(cmd)
	case "qrcode":
		return validateQRCodeCommand(cmd)
	case "feed", "cut", "align", "divider", "folder", "box":
		// These are valid command types with flexible properties
		return nil
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

func validateTextCommand(cmd *Command, variables map[string]bool) error {
	// Must have exactly one of: value, dynamicValue, or arrayField
	count := 0
	if cmd.Value != "" {
		count++
	}
	if cmd.DynamicValue != "" {
		count++
		if !variables[cmd.DynamicValue] {
			return fmt.Errorf("unknown variable '%s' in dynamicValue", cmd.DynamicValue)
		}
	}
	if cmd.ArrayField != "" {
		count++
		if cmd.ArrayBinding == "" {
			return fmt.Errorf("arrayField '%s' used without arrayBinding", cmd.ArrayField)
		}
	}
	
	if count == 0 {
		return fmt.Errorf("text command must have value, dynamicValue, or arrayField")
	}
	if count > 1 {
		return fmt.Errorf("text command cannot have multiple of: value, dynamicValue, arrayField")
	}
	
	// Validate align if present
	if cmd.Align != "" {
		validAligns := []string{"left", "center", "right"}
		valid := false
		for _, a := range validAligns {
			if cmd.Align == a {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid align '%s' (must be left, center, or right)", cmd.Align)
		}
	}
	
	return nil
}

func validateItemCommand(cmd *Command, variables map[string]bool, arrays map[string]bool) error {
	if len(cmd.LeftSide) == 0 {
		return fmt.Errorf("item command requires left_side")
	}
	if len(cmd.RightSide) == 0 {
		return fmt.Errorf("item command requires right_side")
	}
	
	// Validate nested commands
	for i, leftCmd := range cmd.LeftSide {
		if err := validateCommand(&leftCmd, variables, arrays); err != nil {
			return fmt.Errorf("left_side[%d]: %w", i, err)
		}
	}
	for i, rightCmd := range cmd.RightSide {
		if err := validateCommand(&rightCmd, variables, arrays); err != nil {
			return fmt.Errorf("right_side[%d]: %w", i, err)
		}
	}
	
	// Validate width_ratio format if present
	if cmd.WidthRatio != "" {
		parts := strings.Split(cmd.WidthRatio, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid width_ratio '%s' (must be format X:Y)", cmd.WidthRatio)
		}
	}
	
	return nil
}

func validateImageCommand(cmd *Command) error {
	if cmd.Path == "" && cmd.Base64 == "" {
		return fmt.Errorf("image command requires either path or base64")
	}
	if cmd.Path != "" && cmd.Base64 != "" {
		return fmt.Errorf("image command cannot have both path and base64")
	}
	return nil
}

func validateBarcodeCommand(cmd *Command) error {
	if cmd.Value == "" {
		return fmt.Errorf("barcode command requires value")
	}
	
	// Validate format if present
	if cmd.Format != "" {
		validFormats := []string{"EAN13", "EAN8", "CODE39", "CODE128", "UPC_A", "UPC_E", "ITF"}
		valid := false
		for _, f := range validFormats {
			if cmd.Format == f {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid barcode format '%s'", cmd.Format)
		}
	}
	
	return nil
}

func validateQRCodeCommand(cmd *Command) error {
	if cmd.Value == "" {
		return fmt.Errorf("qrcode command requires value")
	}
	
	// Validate error correction if present
	if cmd.ErrorCorrection != "" {
		validLevels := []string{"L", "M", "Q", "H"}
		valid := false
		for _, l := range validLevels {
			if cmd.ErrorCorrection == l {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid error_correction '%s' (must be L, M, Q, or H)", cmd.ErrorCorrection)
		}
	}
	
	return nil
}
