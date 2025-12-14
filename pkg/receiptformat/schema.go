// Package receiptformat defines the types for the .receipt file format
package receiptformat

// Receipt represents the root structure of a .receipt file
type Receipt struct {
	Version        string                    `json:"version"`
	Name           string                    `json:"name,omitempty"`
	Description    string                    `json:"description,omitempty"`
	CreatedWith    string                    `json:"created_with,omitempty"`
	PaperWidth     string                    `json:"paper_width,omitempty"` // "58mm", "80mm", "112mm"
	Font           string                    `json:"font,omitempty"`         // Legacy
	Fonts          map[string]FontFamily     `json:"fonts,omitempty"`
	Variables      []Variable                `json:"variables,omitempty"`
	VariableArrays []VariableArray           `json:"variableArrays,omitempty"`
	Commands       []Command                 `json:"commands"`
}

// FontFamily can be either static or variable
type FontFamily struct {
	Type    string       `json:"type"` // "static" or "variable"
	Path    string       `json:"path,omitempty"`
	Weights []FontWeight `json:"weights,omitempty"`
}

// FontWeight defines a single weight variant for static fonts
type FontWeight struct {
	Weight string `json:"weight"` // thin, light, regular, bold, etc.
	Italic bool   `json:"italic"`
	Path   string `json:"path"`
}

// Variable represents a template variable
type Variable struct {
	Let          string      `json:"let"`
	ValueType    string      `json:"valueType"` // string, number, double, boolean
	DefaultValue interface{} `json:"defaultValue,omitempty"`
	Prefix       string      `json:"prefix,omitempty"`
	Suffix       string      `json:"suffix,omitempty"`
	Description  string      `json:"description,omitempty"`
}

// VariableArray represents a repeatable data structure
type VariableArray struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Schema      []VariableArrayField `json:"schema"`
}

// VariableArrayField defines a field in a variable array
type VariableArrayField struct {
	Field        string      `json:"field"`
	ValueType    string      `json:"valueType"`
	DefaultValue interface{} `json:"defaultValue,omitempty"`
	Prefix       string      `json:"prefix,omitempty"`
	Suffix       string      `json:"suffix,omitempty"`
	Description  string      `json:"description,omitempty"`
}

// Command represents any receipt command
type Command struct {
	Type         string      `json:"type"`
	ArrayBinding string      `json:"arrayBinding,omitempty"`
	
	// Text command
	Value        string `json:"value,omitempty"`
	DynamicValue string `json:"dynamicValue,omitempty"`
	ArrayField   string `json:"arrayField,omitempty"`
	Weight       string `json:"weight,omitempty"`
	Italic       bool   `json:"italic,omitempty"`
	FontFamily   string `json:"font_family,omitempty"`
	Size         int    `json:"size,omitempty"`
	Align        string `json:"align,omitempty"`
	
	// Image command
	Path      string `json:"path,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
	
	// Feed command
	Lines int `json:"lines,omitempty"`
	
	// Item command
	LeftSide     []Command `json:"left_side,omitempty"`
	RightSide    []Command `json:"right_side,omitempty"`
	WidthRatio   string    `json:"width_ratio,omitempty"`
	ShowDivider  bool      `json:"show_divider,omitempty"`
	DividerStyle string    `json:"divider_style,omitempty"`
	
	// Divider command
	Style  string `json:"style,omitempty"`
	Char   string `json:"char,omitempty"`
	Length int    `json:"length,omitempty"`
	
	// Barcode command
	Format   string `json:"format,omitempty"`
	Height   int    `json:"height,omitempty"`
	Width    int    `json:"width,omitempty"`
	Position string `json:"position,omitempty"`
	
	// QR code command
	ErrorCorrection string `json:"error_correction,omitempty"`
	
	// Folder/Box command
	Commands     []Command `json:"commands,omitempty"`
	Title        string    `json:"title,omitempty"`
	Inverted     bool      `json:"inverted,omitempty"`
	Border       int       `json:"border,omitempty"`
	BorderRadius int       `json:"border_radius,omitempty"`
	Padding      int       `json:"padding,omitempty"`
	Margin       int       `json:"margin,omitempty"`
}
