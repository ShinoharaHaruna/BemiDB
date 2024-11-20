package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
	duckDb "github.com/marcboeker/go-duckdb"
	pgQuery "github.com/pganalyze/pg_query_go/v5"
)

const (
	FALLBACK_SQL_QUERY = "SELECT 1"
)

type QueryHandler struct {
	duckdb         *Duckdb
	selectRemapper *SelectRemapper
	config         *Config
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type PreparedStatement struct {
	Name      string
	Query     string
	Statement *sql.Stmt
	Variables []interface{}
	Portal    string
	Rows      *sql.Rows
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type NullDecimal struct {
	Present bool
	Value   duckDb.Decimal
}

func (nullDecimal *NullDecimal) Scan(value interface{}) error {
	if value == nil {
		nullDecimal.Present = false
		return nil
	}

	nullDecimal.Present = true
	nullDecimal.Value = value.(duckDb.Decimal)
	return nil
}

func (nullDecimal NullDecimal) String() string {
	if nullDecimal.Present {
		return fmt.Sprintf("%v", nullDecimal.Value.Float64())
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type NullUint32 struct {
	Present bool
	Value   uint32
}

func (nullUint32 *NullUint32) Scan(value interface{}) error {
	if value == nil {
		nullUint32.Present = false
		return nil
	}

	nullUint32.Present = true
	nullUint32.Value = value.(uint32)
	return nil
}

func (nullUint32 NullUint32) String() string {
	if nullUint32.Present {
		return fmt.Sprintf("%v", nullUint32.Value)
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type NullUint64 struct {
	Present bool
	Value   uint64
}

func (nullUint64 *NullUint64) Scan(value interface{}) error {
	if value == nil {
		nullUint64.Present = false
		return nil
	}

	nullUint64.Present = true
	nullUint64.Value = value.(uint64)
	return nil
}

func (nullUint64 NullUint64) String() string {
	if nullUint64.Present {
		return fmt.Sprintf("%v", nullUint64.Value)
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////////////////////////

type NullArray struct {
	Present bool
	Value   []interface{}
}

func (nullArray *NullArray) Scan(value interface{}) error {
	if value == nil {
		nullArray.Present = false
		return nil
	}

	nullArray.Present = true
	nullArray.Value = value.([]interface{})
	return nil
}

func (nullArray NullArray) String() string {
	if nullArray.Present {
		var stringVals []string
		for _, v := range nullArray.Value {
			switch v.(type) {
			case []uint8:
				stringVals = append(stringVals, fmt.Sprintf("%s", v))
			default:
				stringVals = append(stringVals, fmt.Sprintf("%v", v))
			}
		}
		buffer := &bytes.Buffer{}
		csvWriter := csv.NewWriter(buffer)
		err := csvWriter.Write(stringVals)
		if err != nil {
			return ""
		}
		csvWriter.Flush()
		return "{" + strings.TrimRight(buffer.String(), "\n") + "}"
	}
	return ""
}

////////////////////////////////////////////////////////////////////////////////////////////////////

func NewQueryHandler(config *Config, duckdb *Duckdb, icebergReader *IcebergReader) *QueryHandler {
	ctx := context.Background()

	schemas, err := icebergReader.Schemas()
	PanicIfError(err)
	for _, schema := range schemas {
		_, err := duckdb.ExecContext(
			ctx,
			"CREATE SCHEMA IF NOT EXISTS \"$schema\"",
			map[string]string{"schema": schema},
		)
		PanicIfError(err)
	}

	icebergSchemaTables, err := icebergReader.SchemaTables()
	PanicIfError(err)
	for _, icebergSchemaTable := range icebergSchemaTables {
		metadataFilePath := icebergReader.MetadataFilePath(icebergSchemaTable)
		_, err := duckdb.ExecContext(
			ctx,
			"CREATE VIEW IF NOT EXISTS \"$schema\".\"$table\" AS SELECT * FROM iceberg_scan('$metadataFilePath', skip_schema_inference = true)",
			map[string]string{"schema": icebergSchemaTable.Schema, "table": icebergSchemaTable.Table, "metadataFilePath": metadataFilePath},
		)
		PanicIfError(err)
	}

	return &QueryHandler{
		duckdb:         duckdb,
		selectRemapper: &SelectRemapper{config: config, icebergReader: icebergReader},
		config:         config,
	}
}

func (queryHandler *QueryHandler) HandleQuery(originalQuery string) ([]pgproto3.Message, error) {
	query, err := queryHandler.remapQuery(originalQuery)
	if err != nil {
		LogError(queryHandler.config, "Couldn't map query:", originalQuery+"\n"+err.Error())
		return nil, err
	}

	rows, err := queryHandler.duckdb.QueryContext(context.Background(), query)
	if err != nil {
		LogError(queryHandler.config, "Couldn't handle query via DuckDB:", query+"\n"+err.Error())

		if err.Error() == "Binder Error: UNNEST requires a single list as input" {
			// https://github.com/duckdb/duckdb/issues/11693
			return queryHandler.HandleQuery(FALLBACK_SQL_QUERY)
		}

		return nil, err
	}
	defer rows.Close()

	var messages []pgproto3.Message
	descriptionMessages, err := queryHandler.rowsToDescriptionMessages(rows, query)
	if err != nil {
		return nil, err
	}
	messages = append(messages, descriptionMessages...)
	dataMessages, err := queryHandler.rowsToDataMessages(rows, query)
	if err != nil {
		return nil, err
	}
	messages = append(messages, dataMessages...)
	return messages, nil
}

func (queryHandler *QueryHandler) HandleParseQuery(message *pgproto3.Parse) ([]pgproto3.Message, *PreparedStatement, error) {
	ctx := context.Background()
	originalQuery := string(message.Query)
	query, err := queryHandler.remapQuery(originalQuery)
	if err != nil {
		LogError(queryHandler.config, "Couldn't map query:", originalQuery+"\n"+err.Error())
		return nil, nil, err
	}

	statement, err := queryHandler.duckdb.PrepareContext(ctx, query)
	if err != nil {
		LogError(queryHandler.config, "Couldn't prepare query via DuckDB:", query+"\n"+err.Error())
		return nil, nil, err
	}

	preparedStatement := &PreparedStatement{
		Name:      message.Name,
		Query:     query,
		Statement: statement,
	}

	messages := []pgproto3.Message{&pgproto3.ParseComplete{}}

	return messages, preparedStatement, nil
}

func (queryHandler *QueryHandler) HandleBindQuery(message *pgproto3.Bind, preparedStatement *PreparedStatement) ([]pgproto3.Message, *PreparedStatement, error) {
	if message.PreparedStatement != preparedStatement.Name {
		LogError(queryHandler.config, "Prepared statement mismatch:", message.PreparedStatement, "instead of", preparedStatement.Name)
		return nil, nil, errors.New("Prepared statement mismatch")
	}

	var variables []interface{}
	for _, parameter := range message.Parameters {
		variables = append(variables, string(parameter))
	}

	preparedStatement.Variables = variables
	preparedStatement.Portal = message.DestinationPortal

	messages := []pgproto3.Message{&pgproto3.BindComplete{}}

	return messages, preparedStatement, nil
}

func (queryHandler *QueryHandler) HandleDescribeQuery(message *pgproto3.Describe, preparedStatement *PreparedStatement) ([]pgproto3.Message, *PreparedStatement, error) {
	switch message.ObjectType {
	case 'S': // Statement
		if message.Name != preparedStatement.Query {
			LogError(queryHandler.config, "Statement mismatch:", message.Name, "instead of", preparedStatement.Query)
			return nil, nil, errors.New("Statement mismatch")
		}
	case 'P': // Portal
		if message.Name != preparedStatement.Portal {
			LogError(queryHandler.config, "Portal mismatch:", message.Name, "instead of", preparedStatement.Portal)
			return nil, nil, errors.New("Portal mismatch")
		}
	}

	rows, err := preparedStatement.Statement.QueryContext(context.Background(), preparedStatement.Variables...)
	if err != nil {
		LogError(queryHandler.config, "Couldn't execute prepared statement via DuckDB:", preparedStatement.Query+"\n"+err.Error())
		return nil, nil, err
	}
	preparedStatement.Rows = rows

	messages, err := queryHandler.rowsToDescriptionMessages(preparedStatement.Rows, preparedStatement.Query)
	if err != nil {
		return nil, nil, err
	}
	return messages, preparedStatement, nil
}

func (queryHandler *QueryHandler) HandleExecuteQuery(message *pgproto3.Execute, preparedStatement *PreparedStatement) ([]pgproto3.Message, error) {
	if message.Portal != preparedStatement.Portal {
		LogError(queryHandler.config, "Portal mismatch:", message.Portal, "instead of", preparedStatement.Portal)
		return nil, errors.New("Portal mismatch")
	}

	if preparedStatement.Rows == nil {
		rows, err := preparedStatement.Statement.QueryContext(context.Background(), preparedStatement.Variables...)
		if err != nil {
			LogError(queryHandler.config, "Couldn't execute prepared statement via DuckDB:", preparedStatement.Query+"\n"+err.Error())
			return nil, err
		}
		preparedStatement.Rows = rows
	}

	defer preparedStatement.Rows.Close()

	return queryHandler.rowsToDataMessages(preparedStatement.Rows, preparedStatement.Query)
}

func (queryHandler *QueryHandler) rowsToDescriptionMessages(rows *sql.Rows, query string) ([]pgproto3.Message, error) {
	cols, err := rows.ColumnTypes()
	if err != nil {
		LogError(queryHandler.config, "Couldn't get column types", query+"\n"+err.Error())
		return nil, err
	}

	var messages []pgproto3.Message
	messages = append(messages, queryHandler.generateRowDescription(cols))
	return messages, nil
}

func (queryHandler *QueryHandler) rowsToDataMessages(rows *sql.Rows, query string) ([]pgproto3.Message, error) {
	cols, err := rows.ColumnTypes()
	if err != nil {
		LogError(queryHandler.config, "Couldn't get column types", query+"\n"+err.Error())
		return nil, err
	}

	var messages []pgproto3.Message
	for rows.Next() {
		dataRow, err := queryHandler.generateDataRow(rows, cols)
		if err != nil {
			LogError(queryHandler.config, "Couldn't get data row", query+"\n"+err.Error())
			return nil, err
		}
		messages = append(messages, dataRow)
	}
	messages = append(messages, &pgproto3.CommandComplete{CommandTag: []byte(FALLBACK_SQL_QUERY)})
	return messages, nil
}

func (queryHandler *QueryHandler) remapQuery(query string) (string, error) {
	queryTree, err := pgQuery.Parse(query)
	if err != nil {
		LogError(queryHandler.config, "Error parsing query:", query+"\n"+err.Error())
		return "", err
	}

	var statementNode *pgQuery.Node
	if len(queryTree.Stmts) > 0 {
		statementNode = queryTree.Stmts[0].Stmt
	}

	if statementNode != nil && statementNode.GetSelectStmt() != nil {
		queryTree = queryHandler.selectRemapper.RemapQueryTreeWithSelect(queryTree)
		return pgQuery.Deparse(queryTree)
	}

	if statementNode != nil && statementNode.GetVariableSetStmt() != nil {
		queryTree = queryHandler.selectRemapper.RemapQueryTreeWithSet(queryTree)
		return pgQuery.Deparse(queryTree)
	}

	if statementNode != nil && statementNode.GetDiscardStmt() != nil {
		return FALLBACK_SQL_QUERY, nil
	}

	LogDebug(queryHandler.config, queryTree)
	return "", errors.New("Unsupported query type")
}

func (queryHandler *QueryHandler) generateRowDescription(cols []*sql.ColumnType) *pgproto3.RowDescription {
	description := pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{}}

	for _, col := range cols {
		description.Fields = append(description.Fields, pgproto3.FieldDescription{
			Name:                 []byte(col.Name()),
			TableOID:             0,
			TableAttributeNumber: 0,
			DataTypeOID:          pgtype.TextOID,
			DataTypeSize:         -1,
			TypeModifier:         -1,
			Format:               0,
		})
	}
	return &description
}

func (queryHandler *QueryHandler) generateDataRow(rows *sql.Rows, cols []*sql.ColumnType) (*pgproto3.DataRow, error) {
	valuePtrs := make([]interface{}, len(cols))
	for i, col := range cols {
		switch col.ScanType().String() {
		case "int32":
			var value sql.NullInt32
			valuePtrs[i] = &value
		case "int64", "*big.Int":
			var value sql.NullInt64
			valuePtrs[i] = &value
		case "uint32": // xid
			var value NullUint32
			valuePtrs[i] = &value
		case "uint64": // xid8
			var value NullUint64
			valuePtrs[i] = &value
		case "float64", "float32":
			var value sql.NullFloat64
			valuePtrs[i] = &value
		case "string", "[]uint8": // []uint8 is for uuid
			var value sql.NullString
			valuePtrs[i] = &value
		case "bool":
			var value sql.NullBool
			valuePtrs[i] = &value
		case "time.Time":
			var value sql.NullTime
			valuePtrs[i] = &value
		case "duckdb.Decimal":
			var value NullDecimal
			valuePtrs[i] = &value
		case "[]interface {}":
			var value NullArray
			valuePtrs[i] = &value
		default:
			panic("Unsupported queried type: " + col.ScanType().String())
		}
	}

	err := rows.Scan(valuePtrs...)
	if err != nil {
		return nil, err
	}

	var values [][]byte
	for i, valuePtr := range valuePtrs {
		switch value := valuePtr.(type) {
		case *sql.NullInt32:
			if value.Valid {
				values = append(values, []byte(strconv.Itoa(int(value.Int32))))
			} else {
				values = append(values, nil)
			}
		case *sql.NullInt64:
			if value.Valid {
				values = append(values, []byte(strconv.Itoa(int(value.Int64))))
			} else {
				values = append(values, nil)
			}
		case *NullUint32:
			if value.Present {
				values = append(values, []byte(value.String()))
			} else {
				values = append(values, nil)
			}
		case *NullUint64:
			if value.Present {
				values = append(values, []byte(value.String()))
			} else {
				values = append(values, nil)
			}
		case *sql.NullFloat64:
			if value.Valid {
				values = append(values, []byte(fmt.Sprintf("%v", value.Float64)))
			} else {
				values = append(values, nil)
			}
		case *sql.NullString:
			if value.Valid {
				values = append(values, []byte(value.String))
			} else {
				values = append(values, nil)
			}
		case *sql.NullBool:
			if value.Valid {
				values = append(values, []byte(fmt.Sprintf("%v", value.Bool)))
			} else {
				values = append(values, nil)
			}
		case *sql.NullTime:
			if value.Valid {
				switch cols[i].DatabaseTypeName() {
				case "DATE":
					values = append(values, []byte(value.Time.Format("2006-01-02")))
				case "TIME":
					values = append(values, []byte(value.Time.Format("15:04:05.999999")))
				case "TIMESTAMP":
					values = append(values, []byte(value.Time.Format("2006-01-02 15:04:05.999999")))
				default:
					panic("Unsupported type: " + cols[i].DatabaseTypeName())
				}
			} else {
				values = append(values, nil)
			}
		case *NullDecimal:
			if value.Present {
				values = append(values, []byte(value.String()))
			} else {
				values = append(values, nil)
			}
		case *NullArray:
			if value.Present {
				values = append(values, []byte(value.String()))
			} else {
				values = append(values, nil)
			}
		case *string:
			values = append(values, []byte(*value))
		default:
			panic("Unsupported type: " + cols[i].ScanType().Name())
		}
	}
	dataRow := pgproto3.DataRow{Values: values}

	return &dataRow, nil
}