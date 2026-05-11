package types

import (
	"fmt"
	"strings"

	schemasv1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
)

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

var unquotedDefaultKeywords = map[string]struct{}{
	"CURRENT_TIMESTAMP": {},
	"CURRENT_DATE":      {},
	"CURRENT_TIME":      {},
	"LOCALTIMESTAMP":    {},
	"LOCALTIME":         {},
	"TRUE":              {},
	"FALSE":             {},
	"NULL":              {},
}

func isEnumColumnType(columnType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(columnType))
	return strings.HasPrefix(normalized, "enum(") || strings.HasPrefix(normalized, "enum (")
}

func ShouldQuoteDefaultValue(defaultValue string) bool {
	trimmed := strings.TrimSpace(defaultValue)
	if strings.Contains(trimmed, "(") {
		return false
	}

	if _, ok := unquotedDefaultKeywords[strings.ToUpper(trimmed)]; ok {
		return false
	}

	return true
}

func ShouldQuoteDefaultValueForType(columnType string, defaultValue string) bool {
	if isEnumColumnType(columnType) {
		return true
	}

	return ShouldQuoteDefaultValue(defaultValue)
}

func FormatDefaultValue(defaultValue string) string {
	if ShouldQuoteDefaultValue(defaultValue) {
		return fmt.Sprintf("'%s'", defaultValue)
	}

	return defaultValue
}

func FormatDefaultValueForType(columnType string, defaultValue string) string {
	if ShouldQuoteDefaultValueForType(columnType, defaultValue) {
		return fmt.Sprintf("'%s'", defaultValue)
	}

	return defaultValue
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
