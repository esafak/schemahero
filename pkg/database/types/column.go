package types

import (
	"strings"

	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
)

// ShouldQuoteDefaultValue returns false if the default value is a SQL function
// call, keyword, or expression that should not be quoted as a string literal.
// SQL function calls (e.g. now(), uuid_generate_v4()) and SQL keywords
// (e.g. CURRENT_TIMESTAMP, TRUE, FALSE) must not be quoted, otherwise the
// database will treat them as literal strings instead of evaluating them.
func ShouldQuoteDefaultValue(value string) bool {
	// Don't quote values that look like SQL function calls or expressions
	// containing parentheses (e.g. now(), gen_random_uuid(), (now() + interval '1 day'))
	if strings.Contains(value, "(") {
		return false
	}

	// Don't quote SQL keywords/constants that should be passed as-is.
	// NOTE: "USER" is intentionally excluded because it collides with common
	// enum/data values. Use CURRENT_USER, SESSION_USER, or SYSTEM_USER for
	// the SQL session-user functions.
	upper := strings.ToUpper(value)
	switch upper {
	case "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME",
		"CURRENT_USER", "SESSION_USER", "SYSTEM_USER",
		"NULL", "TRUE", "FALSE":
		return false
	}

	return true
}

// ShouldQuoteDefaultValueForType returns whether the default value should be
// quoted as a string literal, taking the column data type into account.
//
// Enum columns always have their defaults quoted because enum member values
// are string literals by definition — even if they collide with SQL keywords
// (e.g. "user", "true", "false", "null").
func ShouldQuoteDefaultValueForType(value, dataType string) bool {
	if strings.HasPrefix(strings.ToLower(dataType), "enum") {
		return true
	}
	return ShouldQuoteDefaultValue(value)
}

// FormatDefaultValue returns the default value formatted for use in a SQL DDL
// statement, quoting it as a string literal only when appropriate.
func FormatDefaultValue(value string) string {
	if ShouldQuoteDefaultValue(value) {
		return "'" + value + "'"
	}
	return value
}

// FormatDefaultValueForType is like FormatDefaultValue but takes the column
// data type into account so that enum member defaults are always quoted.
func FormatDefaultValueForType(value, dataType string) string {
	if ShouldQuoteDefaultValueForType(value, dataType) {
		return "'" + value + "'"
	}
	return value
}

type ColumnConstraints struct {
	NotNull *bool
}

type ColumnAttributes struct {
	AutoIncrement *bool
}

func BoolsEqual(a, b *bool) bool {
	if a == nil || !*a {
		return b == nil || !*b
	}
	return b != nil && *b
}

type Column struct {
	Name          string
	DataType      string
	ColumnDefault *string
	Constraints   *ColumnConstraints
	Attributes    *ColumnAttributes
	IsArray       bool
	Charset       string
	Collation     string
	IsStatic      bool
}

func ColumnToMysqlSchemaColumn(column *Column) (*schemasv1alpha4.MysqlTableColumn, error) {
	schemaColumn := &schemasv1alpha4.MysqlTableColumn{
		Name: column.Name,
		Type: column.DataType,
	}

	if column.Constraints != nil {
		schemaColumn.Constraints = &schemasv1alpha4.MysqlTableColumnConstraints{
			NotNull: column.Constraints.NotNull,
		}
	}

	if column.Attributes != nil {
		schemaColumn.Attributes = &schemasv1alpha4.MysqlTableColumnAttributes{
			AutoIncrement: column.Attributes.AutoIncrement,
		}
	}

	schemaColumn.Default = column.ColumnDefault

	schemaColumn.Charset = column.Charset
	schemaColumn.Collation = column.Collation

	return schemaColumn, nil
}

func ColumnToPostgresqlSchemaColumn(column *Column) (*schemasv1alpha4.PostgresqlTableColumn, error) {
	schemaColumn := &schemasv1alpha4.PostgresqlTableColumn{
		Name: column.Name,
		Type: column.DataType,
	}

	if column.Constraints != nil {
		schemaColumn.Constraints = &schemasv1alpha4.PostgresqlTableColumnConstraints{
			NotNull: column.Constraints.NotNull,
		}
	}

	if column.Attributes != nil {
		schemaColumn.Attributes = &schemasv1alpha4.PostgresqlTableColumnAttributes{
			AutoIncrement: column.Attributes.AutoIncrement,
		}
	}

	schemaColumn.Default = column.ColumnDefault

	return schemaColumn, nil
}

func ColumnToRqliteSchemaColumn(column *Column) (*schemasv1alpha4.RqliteTableColumn, error) {
	schemaColumn := &schemasv1alpha4.RqliteTableColumn{
		Name: column.Name,
		Type: column.DataType,
	}

	if column.Constraints != nil {
		schemaColumn.Constraints = &schemasv1alpha4.RqliteTableColumnConstraints{
			NotNull: column.Constraints.NotNull,
		}
	}

	if column.Attributes != nil {
		schemaColumn.Attributes = &schemasv1alpha4.RqliteTableColumnAttributes{
			AutoIncrement: column.Attributes.AutoIncrement,
		}
	}

	schemaColumn.Default = column.ColumnDefault

	return schemaColumn, nil
}

func ColumnToSqliteSchemaColumn(column *Column) (*schemasv1alpha4.SqliteTableColumn, error) {
	schemaColumn := &schemasv1alpha4.SqliteTableColumn{
		Name: column.Name,
		Type: column.DataType,
	}

	if column.Constraints != nil {
		schemaColumn.Constraints = &schemasv1alpha4.SqliteTableColumnConstraints{
			NotNull: column.Constraints.NotNull,
		}
	}

	schemaColumn.Default = column.ColumnDefault

	return schemaColumn, nil
}
