package postgrescfg

import (
	"reflect"
	"testing"
	"time"
)

func TestValidateParameterValue(t *testing.T) {
	tests := []struct {
		name    string
		spec    PostgresParameterSpec
		value   string
		wantErr bool
	}{
		// Int parameter tests
		{
			name: "valid int within range",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "0",
				MaxValue:      "100",
			},
			value:   "50",
			wantErr: false,
		},
		{
			name: "int below minimum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			value:   "5",
			wantErr: true,
		},
		{
			name: "int above maximum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "0",
				MaxValue:      "100",
			},
			value:   "150",
			wantErr: true,
		},
		{
			name: "invalid int format",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "0",
				MaxValue:      "100",
			},
			value:   "abc",
			wantErr: true,
		},
		{
			name: "int with invalid min value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "invalid",
				MaxValue:      "100",
			},
			value:   "50",
			wantErr: true,
		},
		{
			name: "int with invalid max value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "0",
				MaxValue:      "invalid",
			},
			value:   "50",
			wantErr: true,
		},

		// Float parameter tests
		{
			name: "valid float within range",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.0",
				MaxValue:      "10.0",
			},
			value:   "5.5",
			wantErr: false,
		},
		{
			name: "float below minimum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "1.0",
				MaxValue:      "10.0",
			},
			value:   "0.5",
			wantErr: true,
		},
		{
			name: "float above maximum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.0",
				MaxValue:      "5.0",
			},
			value:   "7.5",
			wantErr: true,
		},
		{
			name: "float with invalid min value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "invalid",
				MaxValue:      "10.0",
			},
			value:   "5.5",
			wantErr: true,
		},
		{
			name: "float with invalid max value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.0",
				MaxValue:      "invalid",
			},
			value:   "5.5",
			wantErr: true,
		},
		{
			name: "float with empty min and max values",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "",
				MaxValue:      "",
			},
			value:   "5.5",
			wantErr: false,
		},

		// Bytes parameter tests
		{
			name: "valid bytes within range",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "1MB",
				MaxValue:      "1GB",
			},
			value:   "100MB",
			wantErr: false,
		},
		{
			name: "bytes below minimum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "10MB",
				MaxValue:      "1GB",
			},
			value:   "5MB",
			wantErr: true,
		},
		{
			name: "bytes above maximum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "1MB",
				MaxValue:      "100MB",
			},
			value:   "200MB",
			wantErr: true,
		},
		{
			name: "invalid bytes format",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "1MB",
				MaxValue:      "1GB",
			},
			value:   "invalid",
			wantErr: true,
		},
		{
			name: "bytes with different units",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "1kB",
				MaxValue:      "1TB",
			},
			value:   "1GB",
			wantErr: false,
		},
		{
			name: "bytes with invalid min value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "invalid",
				MaxValue:      "1GB",
			},
			value:   "100MB",
			wantErr: true,
		},
		{
			name: "bytes with invalid max value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "1MB",
				MaxValue:      "invalid",
			},
			value:   "100MB",
			wantErr: true,
		},

		// Enum parameter tests
		{
			name: "valid enum value",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeEnum,
				Values:        []string{"on", "off", "try"},
			},
			value:   "on",
			wantErr: false,
		},
		{
			name: "invalid enum value",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeEnum,
				Values:        []string{"on", "off", "try"},
			},
			value:   "invalid",
			wantErr: true,
		},
		{
			name: "enum with empty values",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeEnum,
				Values:        []string{},
			},
			value:   "any",
			wantErr: true,
		},

		// Duration parameter tests
		{
			name: "valid duration within range",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "30s",
				MaxValue:      "24h",
			},
			value:   "5min",
			wantErr: false,
		},
		{
			name: "duration below minimum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "1min",
				MaxValue:      "1h",
			},
			value:   "30s",
			wantErr: true,
		},
		{
			name: "duration above maximum",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "1min",
				MaxValue:      "1h",
			},
			value:   "2h",
			wantErr: true,
		},
		{
			name: "invalid duration format",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "30s",
				MaxValue:      "24h",
			},
			value:   "invalid",
			wantErr: true,
		},
		{
			name: "duration with invalid min value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "invalid",
				MaxValue:      "24h",
			},
			value:   "5min",
			wantErr: true,
		},
		{
			name: "duration with invalid max value in spec",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "30s",
				MaxValue:      "invalid",
			},
			value:   "5min",
			wantErr: true,
		},

		// String parameter tests
		{
			name: "valid string",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
			},
			value:   "some value",
			wantErr: false,
		},
		{
			name: "empty string",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
			},
			value:   "",
			wantErr: true,
		},
		{
			name: "empty string allowed",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
				AllowEmpty:    true,
			},
			value:   "",
			wantErr: false,
		},
		{
			name: "whitespace only string",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
			},
			value:   "   ",
			wantErr: true,
		},
		{
			name: "string with embedded newline rejected",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
			},
			value:   "evil'\nlog_statement = 'all",
			wantErr: true,
		},
		{
			name: "string with carriage return rejected",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
			},
			value:   "foo\rbar",
			wantErr: true,
		},

		// Boolean parameter tests
		{
			name: "valid boolean true",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "true",
			wantErr: false,
		},
		{
			name: "valid boolean false",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "false",
			wantErr: false,
		},
		{
			name: "valid boolean on",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "on",
			wantErr: false,
		},
		{
			name: "valid boolean off",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "off",
			wantErr: false,
		},
		{
			name: "valid boolean yes",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "yes",
			wantErr: false,
		},
		{
			name: "valid boolean no",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "no",
			wantErr: false,
		},
		{
			name: "valid boolean 1",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "1",
			wantErr: false,
		},
		{
			name: "valid boolean 0",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "0",
			wantErr: false,
		},
		{
			name: "invalid boolean value",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBoolean,
			},
			value:   "maybe",
			wantErr: true,
		},

		{
			name: "unknown parameter type",
			spec: PostgresParameterSpec{
				ParameterType: ParameterType(99), // Unknown type
			},
			value:   "any",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameterValue(tt.spec, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParameterValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"1B", 1, false},
		{"1kB", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1TB", 1024 * 1024 * 1024 * 1024, false},
		{"0B", 0, false},
		{"1.5MB", int64(1.5 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"1xb", 0, true},
		{"", 0, true},
		{"1", 1, false},   // No unit specified
		{"1.5", 1, false}, // No unit specified, truncates to int
		{"1.5kB", int64(1.5 * 1024), false},
		{"-1MB", -1024 * 1024, false}, // Negative values
		{"1.5GB", int64(1.5 * 1024 * 1024 * 1024), false},
		{"abc123", 0, true}, // Invalid format - no number at start
		{"123abc", 0, true}, // Invalid format - unknown unit
		// Test that lowercase units are rejected
		{"1b", 0, true},
		{"1kb", 0, true},
		{"1mb", 0, true},
		{"1gb", 0, true},
		{"1tb", 0, true},
		{"0b", 0, true},
		{"1.5mb", 0, true},
		{"1.5KB", 0, true},
		{"-1mb", 0, true},
		{"1.5gb", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("parseBytes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"10s", 10 * time.Second, false},
		{"5min", 5 * time.Minute, false},
		{"24h", 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"0", 0, false},
		{"invalid", 0, true},
		{"1x", 0, true},
		{"", 0, true},
		{"1h30m", 90 * time.Minute, false},       // Go duration format
		{"1.5h", 90 * time.Minute, false},        // Go duration format
		{"1MIN", 1 * time.Minute, false},         // Case insensitive
		{"1H", 1 * time.Hour, false},             // Case insensitive
		{"1D", 24 * time.Hour, false},            // Case insensitive
		{"1.5min", 90 * time.Second, false},      // Fractional minutes
		{"1.5h", 90 * time.Minute, false},        // Fractional hours
		{"1.5d", 36 * time.Hour, false},          // Fractional days
		{"-1h", -1 * time.Hour, false},           // Negative values
		{"1.5s", 1500 * time.Millisecond, false}, // Fractional seconds
		{"abc123", 0, true},                      // Invalid format - no number at start
		{"123abc", 0, true},                      // Invalid format - unknown unit
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("parseDuration() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		settings     map[string]string
		wantErrors   bool
		errorCount   int
	}{
		{
			name:         "valid settings for xata.micro",
			instanceType: "xata.micro",
			settings: map[string]string{
				"max_connections": "25",
				"shared_buffers":  "128MB",
				"work_mem":        "1MB",
			},
			wantErrors: false,
		},
		{
			name:         "invalid max_connections for xata.micro (exceeds limit)",
			instanceType: "xata.micro",
			settings: map[string]string{
				"max_connections": "200", // xata.micro has max of 150
			},
			wantErrors: true,
			errorCount: 1,
		},
		{
			name:         "invalid shared_buffers format",
			instanceType: "xata.small",
			settings: map[string]string{
				"shared_buffers": "invalid_format",
			},
			wantErrors: true,
			errorCount: 1,
		},
		{
			name:         "unknown parameter",
			instanceType: "xata.medium",
			settings: map[string]string{
				"unknown_param": "some_value",
			},
			wantErrors: true,
			errorCount: 1,
		},
		{
			name:         "multiple validation errors",
			instanceType: "xata.small",
			settings: map[string]string{
				"max_connections": "250", // xata.small has max of 200
				"work_mem":        "invalid",
				"unknown_param":   "value",
			},
			wantErrors: true,
			errorCount: 3,
		},
		{
			name:         "valid enum value",
			instanceType: "xata.large",
			settings: map[string]string{
				"huge_pages": "on",
			},
			wantErrors: false,
		},
		{
			name:         "invalid enum value",
			instanceType: "xata.large",
			settings: map[string]string{
				"huge_pages": "invalid_enum_value",
			},
			wantErrors: true,
			errorCount: 1,
		},
		{
			name:         "empty settings",
			instanceType: "xata.micro",
			settings:     map[string]string{},
			wantErrors:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErrors, err := ValidateSettings(tt.instanceType, tt.settings, 0, "", nil)
			// Check for instance type error
			if err != nil {
				if tt.name == "invalid instance type" {
					return // Expected error
				}
				t.Errorf("ValidateSettings() unexpected error = %v", err)
				return
			}

			// Check validation errors
			if tt.wantErrors {
				if validationErrors == nil {
					t.Errorf("ValidateSettings() expected validation errors but got none")
					return
				}
				if tt.errorCount > 0 && len(validationErrors) != tt.errorCount {
					t.Errorf("ValidateSettings() expected %d validation errors, got %d", tt.errorCount, len(validationErrors))
				}
			} else if validationErrors != nil {
				t.Errorf("ValidateSettings() unexpected validation errors: %v", validationErrors)
			}
		})
	}
}

func TestValidateSettings_InvalidInstanceType(t *testing.T) {
	settings := map[string]string{
		"max_connections": "25",
	}

	validationErrors, err := ValidateSettings("invalid.instance.type", settings, 0, "", nil)

	if err == nil {
		t.Errorf("ValidateSettings() expected error for invalid instance type but got none")
	}

	if validationErrors != nil {
		t.Errorf("ValidateSettings() expected no validation errors when instance type is invalid, got %v", validationErrors)
	}
}

func TestValidateSettings_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		settings     map[string]string
		description  string
	}{
		{
			name:         "boundary values for xata.micro",
			instanceType: "xata.micro",
			settings: map[string]string{
				"max_connections": "50", // Exactly at the limit
				"shared_buffers":  "0B", // Minimum value
			},
			description: "should accept boundary values",
		},
		{
			name:         "boundary values for xata.small",
			instanceType: "xata.small",
			settings: map[string]string{
				"max_connections": "100", // Exactly at the limit
				"work_mem":        "1GB", // Maximum value
			},
			description: "should accept boundary values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationErrors, err := ValidateSettings(tt.instanceType, tt.settings, 0, "", nil)
			if err != nil {
				t.Errorf("ValidateSettings() unexpected error = %v", err)
				return
			}

			if validationErrors != nil {
				t.Errorf("ValidateSettings() unexpected validation errors: %v", validationErrors)
			}
		})
	}
}

func TestAdjustValueToBounds(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		spec     PostgresParameterSpec
		expected string
	}{
		// Integer tests
		{
			name:  "int within bounds",
			value: "50",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			expected: "50",
		},
		{
			name:  "int below min",
			value: "5",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			expected: "10",
		},
		{
			name:  "int above max",
			value: "150",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			expected: "100",
		},
		{
			name:  "int only min bound",
			value: "5",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
			},
			expected: "10",
		},
		{
			name:  "int only max bound",
			value: "150",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MaxValue:      "100",
			},
			expected: "100",
		},
		{
			name:  "int no bounds",
			value: "50",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
			},
			expected: "50",
		},

		// Float tests
		{
			name:  "float within bounds",
			value: "0.5",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.1",
				MaxValue:      "1.0",
			},
			expected: "0.5",
		},
		{
			name:  "float below min",
			value: "0.05",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.1",
				MaxValue:      "1.0",
			},
			expected: "0.1",
		},
		{
			name:  "float above max",
			value: "1.5",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.1",
				MaxValue:      "1.0",
			},
			expected: "1.0",
		},

		// Bytes tests
		{
			name:  "bytes within bounds",
			value: "512MB",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "256MB",
				MaxValue:      "1GB",
			},
			expected: "512MB",
		},
		{
			name:  "bytes below min",
			value: "128MB",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "256MB",
				MaxValue:      "1GB",
			},
			expected: "256MB",
		},
		{
			name:  "bytes above max",
			value: "2GB",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "256MB",
				MaxValue:      "1GB",
			},
			expected: "1GB",
		},

		// Duration tests
		{
			name:  "duration within bounds",
			value: "30s",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "10s",
				MaxValue:      "60s",
			},
			expected: "30s",
		},
		{
			name:  "duration below min",
			value: "5s",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "10s",
				MaxValue:      "60s",
			},
			expected: "10s",
		},
		{
			name:  "duration above max",
			value: "90s",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "10s",
				MaxValue:      "60s",
			},
			expected: "60s",
		},

		// String and enum tests (no adjustment)
		{
			name:  "string type no bounds",
			value: "some_value",
			spec: PostgresParameterSpec{
				ParameterType: ParameterTypeString,
				MinValue:      "min",
				MaxValue:      "max",
			},
			expected: "some_value",
		},
		{
			name:  "enum type no bounds",
			value: "on",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeEnum,
				MinValue:      "min",
				MaxValue:      "max",
			},
			expected: "on",
		},

		// Edge cases
		{
			name:  "invalid int value",
			value: "not_a_number",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			expected: "not_a_number",
		},
		{
			name:  "invalid float value",
			value: "not_a_float",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeFloat,
				MinValue:      "0.1",
				MaxValue:      "1.0",
			},
			expected: "not_a_float",
		},
		{
			name:  "invalid bytes value",
			value: "invalid_bytes",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeBytes,
				MinValue:      "256MB",
				MaxValue:      "1GB",
			},
			expected: "invalid_bytes",
		},
		{
			name:  "invalid duration value",
			value: "invalid_duration",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeDuration,
				MinValue:      "10s",
				MaxValue:      "60s",
			},
			expected: "invalid_duration",
		},
		{
			name:  "invalid min value in spec",
			value: "50",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "invalid_min",
				MaxValue:      "100",
			},
			expected: "50",
		},
		{
			name:  "invalid max value in spec",
			value: "50",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "invalid_max",
			},
			expected: "50",
		},
		{
			name:  "empty value",
			value: "",
			spec: PostgresParameterSpec{
				ParameterType: ParamTypeInt,
				MinValue:      "10",
				MaxValue:      "100",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdjustValueToBounds(tt.value, tt.spec)
			if result != tt.expected {
				t.Errorf("AdjustValueToBounds(%q, %+v) = %q, want %q", tt.value, tt.spec, result, tt.expected)
			}
		})
	}
}

func TestFilterConfigurableParameters(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "all configurable parameters",
			input: map[string]string{
				"max_connections": "100",
				"shared_buffers":  "256mb",
				"work_mem":        "1mb",
			},
			expected: map[string]string{
				"max_connections": "100",
				"shared_buffers":  "256mb",
				"work_mem":        "1mb",
			},
		},
		{
			name: "mixed configurable and non-configurable parameters",
			input: map[string]string{
				"max_connections": "100",
				"shared_buffers":  "256mb",
				"unknown_param":   "some_value",
				"another_unknown": "another_value",
			},
			expected: map[string]string{
				"max_connections": "100",
				"shared_buffers":  "256mb",
			},
		},
		{
			name: "no configurable parameters",
			input: map[string]string{
				"unknown_param1": "value1",
				"unknown_param2": "value2",
			},
			expected: map[string]string{},
		},
		{
			name:     "empty input",
			input:    map[string]string{},
			expected: map[string]string{},
		},
		{
			name:     "nil input",
			input:    nil,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterConfigurableParameters(tt.input, 0, "", nil)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("FilterConfigurableParameters() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetConfigurableParameters_ExtensionFiltering(t *testing.T) {
	tests := []struct {
		name         string
		majorVersion int
		image        string
		paramName    string
		shouldExist  bool
	}{
		{
			name:         "duckdb.postgres_role available for analytics:17",
			majorVersion: 17,
			image:        "analytics:17",
			paramName:    "duckdb.postgres_role",
			shouldExist:  true,
		},
		{
			name:         "duckdb.postgres_role not available for postgres:17",
			majorVersion: 17,
			image:        "postgres:17",
			paramName:    "duckdb.postgres_role",
			shouldExist:  false,
		},
		{
			name:         "duckdb.postgres_role available when no image filter",
			majorVersion: 17,
			image:        "",
			paramName:    "duckdb.postgres_role",
			shouldExist:  true,
		},
		{
			name:         "pg_stat_statements.max available for postgres:17",
			majorVersion: 17,
			image:        "postgres:17",
			paramName:    "pg_stat_statements.max",
			shouldExist:  true,
		},
		{
			name:         "pg_stat_statements.max available for analytics:17",
			majorVersion: 17,
			image:        "analytics:17",
			paramName:    "pg_stat_statements.max",
			shouldExist:  true,
		},
		{
			name:         "pg_stat_statements.max available for full postgres image URL",
			majorVersion: 17,
			image:        "ghcr.io/xataio/postgres-images/cnpg-postgres-plus:17.5",
			paramName:    "pg_stat_statements.max",
			shouldExist:  true,
		},
		{
			name:         "duckdb.postgres_role available for full analytics image URL",
			majorVersion: 17,
			image:        "ghcr.io/xataio/postgres-images/xata-analytics:17.5",
			paramName:    "duckdb.postgres_role",
			shouldExist:  true,
		},
		{
			name:         "duckdb.postgres_role not available for full postgres image URL",
			majorVersion: 17,
			image:        "ghcr.io/xataio/postgres-images/cnpg-postgres-plus:17.5",
			paramName:    "duckdb.postgres_role",
			shouldExist:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GetConfigurableParameters(tt.majorVersion, tt.image, nil)
			_, exists := params[tt.paramName]
			if exists != tt.shouldExist {
				if tt.shouldExist {
					t.Errorf("%s should be available for image %q", tt.paramName, tt.image)
				} else {
					t.Errorf("%s should NOT be available for image %q", tt.paramName, tt.image)
				}
			}
		})
	}
}

func TestGetConfigurableParameters_PreloadFiltering(t *testing.T) {
	tests := []struct {
		name             string
		majorVersion     int
		image            string
		preloadLibraries []string
		paramName        string
		shouldExist      bool
	}{
		{
			name:             "pg_stat_statements.max available when preloaded",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{"pg_stat_statements"},
			paramName:        "pg_stat_statements.max",
			shouldExist:      true,
		},
		{
			name:             "pg_stat_statements.max not available when not preloaded",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{},
			paramName:        "pg_stat_statements.max",
			shouldExist:      false,
		},
		{
			name:             "pg_stat_statements.max available when preload is nil (no filtering)",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: nil,
			paramName:        "pg_stat_statements.max",
			shouldExist:      true,
		},
		{
			name:             "auto_explain.log_min_duration available when preloaded",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{"auto_explain"},
			paramName:        "auto_explain.log_min_duration",
			shouldExist:      true,
		},
		{
			name:             "auto_explain.log_min_duration not available when not preloaded",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{"pg_stat_statements"},
			paramName:        "auto_explain.log_min_duration",
			shouldExist:      false,
		},
		{
			name:             "non-extension parameter available regardless of preload",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{},
			paramName:        "max_connections",
			shouldExist:      true,
		},
		{
			name:             "multiple extensions preloaded",
			majorVersion:     17,
			image:            "postgres:17",
			preloadLibraries: []string{"pg_stat_statements", "auto_explain"},
			paramName:        "pg_stat_statements.max",
			shouldExist:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := GetConfigurableParameters(tt.majorVersion, tt.image, tt.preloadLibraries)
			_, exists := params[tt.paramName]
			if exists != tt.shouldExist {
				if tt.shouldExist {
					t.Errorf("%s should be available with preload %v", tt.paramName, tt.preloadLibraries)
				} else {
					t.Errorf("%s should NOT be available with preload %v", tt.paramName, tt.preloadLibraries)
				}
			}
		})
	}
}
