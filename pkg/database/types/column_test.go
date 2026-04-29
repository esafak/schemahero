package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolsEqual(t *testing.T) {
	falseValue := false
	trueValue := true
	tests := []struct {
		name string
		a    *bool
		b    *bool
		want bool
	}{
		{
			name: "false false",
			a:    &falseValue,
			b:    &falseValue,
			want: true,
		},
		{
			name: "true true",
			a:    &trueValue,
			b:    &trueValue,
			want: true,
		},
		{
			name: "false true",
			a:    &falseValue,
			b:    &trueValue,
			want: false,
		},
		{
			name: "true false",
			a:    &trueValue,
			b:    &falseValue,
			want: false,
		},
		{
			name: "nil nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "nil false",
			a:    nil,
			b:    &falseValue,
			want: true,
		},
		{
			name: "false nil",
			a:    &falseValue,
			b:    nil,
			want: true,
		},
		{
			name: "nil true",
			a:    nil,
			b:    &trueValue,
			want: false,
		},
		{
			name: "true nil",
			a:    &trueValue,
			b:    nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BoolsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("BoolsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldQuoteDefaultValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		// String literal values should be quoted
		{
			name:  "plain string",
			value: "hello",
			want:  true,
		},
		{
			name:  "numeric string",
			value: "11",
			want:  true,
		},
		{
			name:  "empty string",
			value: "",
			want:  true,
		},
		{
			name:  "text with spaces",
			value: "hello world",
			want:  true,
		},
		// SQL functions should NOT be quoted
		{
			name:  "now() function",
			value: "now()",
			want:  false,
		},
		{
			name:  "gen_random_uuid() function",
			value: "gen_random_uuid()",
			want:  false,
		},
		{
			name:  "uuid_generate_v4() function",
			value: "uuid_generate_v4()",
			want:  false,
		},
		{
			name:  "CURRENT_TIMESTAMP(6)",
			value: "CURRENT_TIMESTAMP(6)",
			want:  false,
		},
		{
			name:  "expression with parentheses",
			value: "(now() + interval '1 day')",
			want:  false,
		},
		// SQL keywords should NOT be quoted
		{
			name:  "CURRENT_TIMESTAMP",
			value: "CURRENT_TIMESTAMP",
			want:  false,
		},
		{
			name:  "current_timestamp lowercase",
			value: "current_timestamp",
			want:  false,
		},
		{
			name:  "CURRENT_DATE",
			value: "CURRENT_DATE",
			want:  false,
		},
		{
			name:  "CURRENT_TIME",
			value: "CURRENT_TIME",
			want:  false,
		},
		{
			name:  "TRUE",
			value: "TRUE",
			want:  false,
		},
		{
			name:  "FALSE",
			value: "FALSE",
			want:  false,
		},
		{
			name:  "NULL",
			value: "NULL",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldQuoteDefaultValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldQuoteDefaultValueForType(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		dataType string
		want     bool
	}{
		// Enum columns always quote their defaults — even values that look
		// like SQL keywords.
		{
			name:     "enum default 'user' is quoted despite USER keyword",
			value:    "user",
			dataType: "enum('admin','user','guest')",
			want:     true,
		},
		{
			name:     "enum default 'true' is quoted despite TRUE keyword",
			value:    "true",
			dataType: "enum('true','false')",
			want:     true,
		},
		{
			name:     "enum default 'false' is quoted despite FALSE keyword",
			value:    "false",
			dataType: "enum('true','false')",
			want:     true,
		},
		{
			name:     "enum default 'null' is quoted despite NULL keyword",
			value:    "null",
			dataType: "enum('null','not_null')",
			want:     true,
		},
		{
			name:     "enum default plain string",
			value:    "active",
			dataType: "enum('active','inactive')",
			want:     true,
		},
		{
			name:     "enum uppercase type name",
			value:    "user",
			dataType: "ENUM('admin','user')",
			want:     true,
		},
		// Non-enum types delegate to ShouldQuoteDefaultValue
		{
			name:     "varchar default 'hello' is quoted",
			value:    "hello",
			dataType: "varchar (255)",
			want:     true,
		},
		{
			name:     "timestamp default CURRENT_TIMESTAMP not quoted",
			value:    "CURRENT_TIMESTAMP",
			dataType: "timestamp",
			want:     false,
		},
		{
			name:     "datetime default now() not quoted",
			value:    "now()",
			dataType: "datetime",
			want:     false,
		},
		{
			name:     "varchar default 'user' is quoted (USER not treated as keyword)",
			value:    "user",
			dataType: "varchar (255)",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldQuoteDefaultValueForType(tt.value, tt.dataType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDefaultValueForType(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		dataType string
		want     string
	}{
		{
			name:     "enum default 'user' is quoted",
			value:    "user",
			dataType: "enum('admin','user','guest')",
			want:     "'user'",
		},
		{
			name:     "enum default 'true' is quoted",
			value:    "true",
			dataType: "enum('true','false')",
			want:     "'true'",
		},
		{
			name:     "timestamp default CURRENT_TIMESTAMP not quoted",
			value:    "CURRENT_TIMESTAMP",
			dataType: "timestamp",
			want:     "CURRENT_TIMESTAMP",
		},
		{
			name:     "datetime default now() not quoted",
			value:    "now()",
			dataType: "datetime",
			want:     "now()",
		},
		{
			name:     "varchar default 'hello' is quoted",
			value:    "hello",
			dataType: "varchar (255)",
			want:     "'hello'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDefaultValueForType(tt.value, tt.dataType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDefaultValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "plain string gets quoted",
			value: "hello",
			want:  "'hello'",
		},
		{
			name:  "numeric string gets quoted",
			value: "11",
			want:  "'11'",
		},
		{
			name:  "now() not quoted",
			value: "now()",
			want:  "now()",
		},
		{
			name:  "CURRENT_TIMESTAMP not quoted",
			value: "CURRENT_TIMESTAMP",
			want:  "CURRENT_TIMESTAMP",
		},
		{
			name:  "gen_random_uuid() not quoted",
			value: "gen_random_uuid()",
			want:  "gen_random_uuid()",
		},
		{
			name:  "empty string gets quoted",
			value: "",
			want:  "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDefaultValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}
