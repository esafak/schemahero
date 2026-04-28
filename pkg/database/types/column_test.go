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
