package main

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestHandleQuery(t *testing.T) {
	var responsesByQuery = map[string]map[string][]string{
		// PG functions
		"SELECT VERSION()": {
			"description": {"version"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"PostgreSQL 17.0, compiled by Bemi"},
		},
		"SELECT pg_catalog.pg_get_userbyid(p.proowner) AS owner, 'Foo' AS foo FROM pg_catalog.pg_proc p LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace LIMIT 1": {
			"description": {"owner", "foo"},
			"types":       {Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb", "Foo"},
		},
		"SELECT QUOTE_IDENT('fooBar')": {
			"description": {"quote_ident"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"\"fooBar\""},
		},
		"SELECT setting from pg_show_all_settings() WHERE name = 'default_null_order'": {
			"description": {"setting"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"nulls_last"},
		},
		"SELECT setting from pg_catalog.pg_show_all_settings() WHERE name = 'default_null_order'": {
			"description": {"setting"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"nulls_last"},
		},
		"SELECT pg_catalog.pg_get_partkeydef(c.oid) FROM pg_catalog.pg_class c LIMIT 1": {
			"description": {"pg_get_partkeydef"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT pg_tablespace_location(t.oid) loc FROM pg_catalog.pg_tablespace": {
			"description": {"loc"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT pg_catalog.pg_get_expr(adbin, drelid, TRUE) AS def_value FROM pg_catalog.pg_attrdef": {
			"description": {"def_value"},
		},
		"SELECT set_config('bytea_output', 'hex', false)": {
			"description": {"set_config"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"hex"},
		},
		"SELECT pg_catalog.pg_encoding_to_char(6)": {
			"description": {"pg_encoding_to_char"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"UTF8"},
		},
		"SELECT pg_backend_pid()": {
			"description": {"pg_backend_pid"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"0"},
		},
		"SELECT * from pg_is_in_recovery()": {
			"description": {"pg_is_in_recovery"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"f"},
		},
		"SELECT row_to_json(t) FROM (SELECT usename, passwd FROM pg_shadow WHERE usename='bemidb') t": {
			"description": {"row_to_json"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {`{"usename":"bemidb","passwd":"bemidb-encrypted"}`},
		},
		"SELECT current_setting('default_tablespace')": {
			"description": {"current_setting"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		// PG system tables
		"SELECT oid, typname AS typename FROM pg_type WHERE typname='geometry' OR typname='geography'": {
			"description": {"oid", "typename"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT relname FROM pg_catalog.pg_class WHERE relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = 'public' LIMIT 1) LIMIT 1": {
			"description": {"relname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"test_table"},
		},
		"SELECT oid FROM pg_catalog.pg_extension": {
			"description": {"oid"},
			"types":       {Uint32ToString(pgtype.OIDOID)},
			"values":      {"13823"},
		},
		"SELECT slot_name FROM pg_replication_slots": {
			"description": {"slot_name"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT oid, datname, datdba FROM pg_catalog.pg_database where oid = 16388": {
			"description": {"oid", "datname", "datdba"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.Int8OID)},
			"values":      {"16388", "bemidb", "10"},
		},
		"SELECT * FROM pg_catalog.pg_stat_gssapi": {
			"description": {"pid", "gss_authenticated", "principal", "encrypted", "credentials_delegated"},
			"types":       {Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT * FROM pg_catalog.pg_user": {
			"description": {"usename", "usesysid", "usecreatedb", "usesuper", "userepl", "usebypassrls", "passwd", "valuntil", "useconfig"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb", "10", "t", "t", "t", "t", "", "NULL", "NULL"},
		},
		"SELECT datid FROM pg_catalog.pg_stat_activity": {
			"description": {"datid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		// pg_namespace
		"SELECT DISTINCT(nspname) FROM pg_catalog.pg_namespace WHERE nspname != 'information_schema' AND nspname != 'pg_catalog'": {
			"description": {"nspname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"public"},
		},
		"SELECT nspname FROM pg_catalog.pg_namespace WHERE nspname == 'main'": {
			"description": {"nspname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT n.nspname FROM pg_catalog.pg_namespace n LEFT OUTER JOIN pg_catalog.pg_description d ON d.objoid = n.oid ORDER BY n.oid LIMIT 1": {
			"description": {"nspname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"public"},
		},
		"SELECT pg_total_relation_size(relid) AS total_size FROM pg_catalog.pg_statio_user_tables WHERE schemaname = 'public'": {
			"description": {"total_size"},
			"types":       {Uint32ToString(pgtype.TextOID)},
		},
		"SELECT pg_total_relation_size(relid) AS total_size FROM pg_catalog.pg_statio_user_tables WHERE schemaname = 'public' UNION SELECT NULL AS total_size FROM pg_catalog.pg_proc p LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'public'": {
			"description": {"total_size"},
			"types":       {Uint32ToString(pgtype.TextOID)},
		},
		"SELECT * FROM pg_catalog.pg_shdescription": {
			"description": {"objoid", "classoid", "description"},
			"types":       {Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID)},
		},
		"SELECT * FROM pg_catalog.pg_roles": {
			"description": {"oid", "rolname", "rolsuper", "rolinherit", "rolcreaterole", "rolcreatedb", "rolcanlogin", "rolreplication", "rolconnlimit", "rolpassword", "rolvaliduntil", "rolbypassrls", "rolconfig"},
			"types": {
				Uint32ToString(pgtype.OIDOID),
				Uint32ToString(pgtype.TextOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.Int8OID),
				Uint32ToString(pgtype.TextOID),
				Uint32ToString(pgtype.TextOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.TextOID),
			},
			"values": {"10", "bemidb", "true", "true", "true", "true", "true", "false", "-1", "NULL", "NULL", "false", "NULL"},
		},
		"SELECT * FROM pg_catalog.pg_inherits": {
			"description": {"inhrelid", "inhparent", "inhseqno", "inhdetachpending"},
		},
		"SELECT * FROM pg_auth_members": {
			"description": {"oid", "roleid", "member", "grantor", "admin_option", "inherit_option", "set_option"},
		},
		// Information schema
		"SELECT * FROM information_schema.tables": {
			"description": {"table_catalog", "table_schema", "table_name", "table_type", "self_referencing_column_name", "reference_generation", "user_defined_type_catalog", "user_defined_type_schema", "user_defined_type_name", "is_insertable_into", "is_typed", "commit_action", "TABLE_COMMENT"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"memory", "public", "test_table", "BASE TABLE", "", "", "", "", "", "YES", "NO", "", ""},
		},
		"SELECT table_catalog, table_schema, table_name AS table FROM information_schema.tables": {
			"description": {"table_catalog", "table_schema", "table"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"memory", "public", "test_table"},
		},
		// Empty query
		"-- ping": {
			"description": {"1"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {"1"},
		},
		// DISCARD
		"DISCARD ALL": {
			"description": {"1"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {"1"},
		},
		// Iceberg data
		"SELECT COUNT(*) AS count FROM public.test_table": {
			"description": {"count"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"2"},
		},
		"SELECT COUNT(*) AS count FROM test_table": {
			"description": {"count"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"2"},
		},
		"SELECT x.bit_column FROM public.test_table x WHERE x.bit_column IS NOT NULL": {
			"description": {"bit_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"1"},
		},
		"SELECT bit_column FROM public.test_table WHERE bit_column IS NOT NULL": {
			"description": {"bit_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"1"},
		},
		"SELECT test_table.bit_column FROM public.test_table WHERE bit_column IS NOT NULL": {
			"description": {"bit_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"1"},
		},
		"SELECT bit_column FROM public.test_table WHERE bit_column IS NULL": {
			"description": {"bit_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT bool_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"bool_column"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {"true"},
		},
		"SELECT bool_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"bool_column"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {"false"},
		},
		"SELECT bpchar_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"bpchar_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bpchar"},
		},
		"SELECT bpchar_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"bpchar_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT varchar_column FROM public.test_table WHERE varchar_column IS NOT NULL": {
			"description": {"varchar_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"varchar"},
		},
		"SELECT varchar_column FROM public.test_table WHERE varchar_column IS NULL": {
			"description": {"varchar_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT text_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"text_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"text"},
		},
		"SELECT text_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"text_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT int2_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"int2_column"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {"32767"},
		},
		"SELECT int2_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"int2_column"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {"-32767"},
		},
		"SELECT int4_column FROM public.test_table WHERE int4_column IS NOT NULL": {
			"description": {"int4_column"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {"2147483647"},
		},
		"SELECT int4_column FROM public.test_table WHERE int4_column IS NULL": {
			"description": {"int4_column"},
			"types":       {Uint32ToString(pgtype.Int4OID)},
			"values":      {""},
		},
		"SELECT int8_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"int8_column"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"9223372036854775807"},
		},
		"SELECT int8_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"int8_column"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"-9223372036854775807"},
		},
		"SELECT xid_column FROM public.test_table WHERE xid_column IS NOT NULL": {
			"description": {"xid_column"},
			"types":       {Uint32ToString(pgtype.XIDOID)},
			"values":      {"4294967295"},
		},
		"SELECT xid_column FROM public.test_table WHERE xid_column IS NULL": {
			"description": {"xid_column"},
			"types":       {Uint32ToString(pgtype.XIDOID)},
			"values":      {""},
		},
		"SELECT xid8_column FROM public.test_table WHERE xid8_column IS NOT NULL": {
			"description": {"xid8_column"},
			"types":       {Uint32ToString(pgtype.XID8OID)},
			"values":      {"18446744073709551615"},
		},
		"SELECT xid8_column FROM public.test_table WHERE xid8_column IS NULL": {
			"description": {"xid8_column"},
			"types":       {Uint32ToString(pgtype.XID8OID)},
			"values":      {""},
		},
		"SELECT float4_column FROM public.test_table WHERE float4_column = 3.14": {
			"description": {"float4_column"},
			"types":       {Uint32ToString(pgtype.Float4OID)},
			"values":      {"3.14"},
		},
		"SELECT float4_column FROM public.test_table WHERE float4_column != 3.14": {
			"description": {"float4_column"},
			"types":       {Uint32ToString(pgtype.Float4OID)},
			"values":      {"NaN"},
		},
		"SELECT float8_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"float8_column"},
			"types":       {Uint32ToString(pgtype.Float8OID)},
			"values":      {"3.141592653589793"},
		},
		"SELECT float8_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"float8_column"},
			"types":       {Uint32ToString(pgtype.Float8OID)},
			"values":      {"-3.141592653589793"},
		},
		"SELECT numeric_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"numeric_column"},
			"types":       {Uint32ToString(pgtype.NumericOID)},
			"values":      {"12345.67"},
		},
		"SELECT numeric_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"numeric_column"},
			"types":       {Uint32ToString(pgtype.NumericOID)},
			"values":      {"-12345"},
		},
		"SELECT date_column FROM public.test_table WHERE date_column IS NOT NULL": {
			"description": {"date_column"},
			"types":       {Uint32ToString(pgtype.DateOID)},
			"values":      {"2021-01-01"},
		},
		"SELECT date_column FROM public.test_table WHERE date_column IS NULL": {
			"description": {"date_column"},
			"types":       {Uint32ToString(pgtype.DateOID)},
			"values":      {""},
		},
		"SELECT time_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"time_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {"12:00:00.123456"},
		},
		"SELECT time_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"time_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {"12:00:00.123"},
		},
		"SELECT time_ms_column FROM public.test_table WHERE time_ms_column IS NOT NULL": {
			"description": {"time_ms_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {"12:00:00.123"},
		},
		"SELECT time_ms_column FROM public.test_table WHERE time_ms_column IS NULL": {
			"description": {"time_ms_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {""},
		},
		"SELECT timetz_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"timetz_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {"17:00:00.123456"},
		},
		"SELECT timetz_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"timetz_column"},
			"types":       {Uint32ToString(pgtype.TimeOID)},
			"values":      {"07:00:00.123"},
		},
		"SELECT timestamp_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"timestamp_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 12:00:00.123456"},
		},
		"SELECT timestamp_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"timestamp_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 12:00:00"},
		},
		"SELECT timestamp_ms_column FROM public.test_table WHERE timestamp_ms_column IS NOT NULL": {
			"description": {"timestamp_ms_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 12:00:00.123"},
		},
		"SELECT timestamp_ms_column FROM public.test_table WHERE timestamp_ms_column IS NULL": {
			"description": {"timestamp_ms_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {""},
		},
		"SELECT timestamptz_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"timestamptz_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 17:00:00.123456"},
		},
		"SELECT timestamptz_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"timestamptz_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 07:00:00.000123"},
		},
		"SELECT timestamptz_ms_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"timestamptz_ms_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 17:00:00.123"},
		},
		"SELECT timestamptz_ms_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"timestamptz_ms_column"},
			"types":       {Uint32ToString(pgtype.TimestampOID)},
			"values":      {"2024-01-01 07:00:00.12"},
		},
		"SELECT uuid_column FROM public.test_table WHERE uuid_column IS NOT NULL": {
			"description": {"uuid_column"},
			"types":       {Uint32ToString(pgtype.ByteaOID)},
			"values":      {"58a7c845-af77-44b2-8664-7ca613d92f04"},
		},
		"SELECT uuid_column FROM public.test_table WHERE uuid_column IS NULL": {
			"description": {"uuid_column"},
			"types":       {Uint32ToString(pgtype.ByteaOID)},
			"values":      {""},
		},
		"SELECT bytea_column FROM public.test_table WHERE bytea_column IS NOT NULL": {
			"description": {"bytea_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"\\x1234"},
		},
		"SELECT bytea_column FROM public.test_table WHERE bytea_column IS NULL": {
			"description": {"bytea_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT interval_column FROM public.test_table WHERE interval_column IS NOT NULL": {
			"description": {"interval_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"1 mon 2 days 01:00:01.000001"},
		},
		"SELECT interval_column FROM public.test_table WHERE interval_column IS NULL": {
			"description": {"interval_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT json_column FROM public.test_table WHERE json_column IS NOT NULL": {
			"description": {"json_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"{\"key\": \"value\"}"},
		},
		"SELECT json_column FROM public.test_table WHERE json_column IS NULL": {
			"description": {"json_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT jsonb_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"jsonb_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"{\"key\": \"value\"}"},
		},
		"SELECT jsonb_column->'key' AS key FROM public.test_table WHERE jsonb_column->>'key' = 'value'": {
			"description": {"key"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"\"value\""},
		},
		"SELECT jsonb_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"jsonb_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"{}"},
		},
		"SELECT tsvector_column FROM public.test_table WHERE tsvector_column IS NOT NULL": {
			"description": {"tsvector_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"'sampl':1 'text':2 'tsvector':4"},
		},
		"SELECT tsvector_column FROM public.test_table WHERE tsvector_column IS NULL": {
			"description": {"tsvector_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT xml_column FROM public.test_table WHERE xml_column IS NOT NULL": {
			"description": {"xml_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"<root><child>text</child></root>"},
		},
		"SELECT xml_column FROM public.test_table WHERE xml_column IS NULL": {
			"description": {"xml_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT pg_snapshot_column FROM public.test_table WHERE pg_snapshot_column IS NOT NULL": {
			"description": {"pg_snapshot_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"2784:2784:"},
		},
		"SELECT pg_snapshot_column FROM public.test_table WHERE pg_snapshot_column IS NULL": {
			"description": {"pg_snapshot_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT point_column FROM public.test_table WHERE point_column IS NOT NULL": {
			"description": {"point_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"(37.347301483154,45.002101898193)"},
		},
		"SELECT point_column FROM public.test_table WHERE point_column IS NULL": {
			"description": {"point_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT inet_column FROM public.test_table WHERE inet_column IS NOT NULL": {
			"description": {"inet_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"192.168.0.1"},
		},
		"SELECT inet_column FROM public.test_table WHERE inet_column IS NULL": {
			"description": {"inet_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT array_text_column FROM public.test_table WHERE array_text_column IS NOT NULL": {
			"description": {"array_text_column"},
			"types":       {Uint32ToString(pgtype.TextArrayOID)},
			"values":      {"{one,two,three}"},
		},
		"SELECT array_text_column FROM public.test_table WHERE array_text_column IS NULL": {
			"description": {"array_text_column"},
			"types":       {Uint32ToString(pgtype.TextArrayOID)},
			"values":      {""},
		},
		"SELECT array_int_column FROM public.test_table WHERE bool_column = TRUE": {
			"description": {"array_int_column"},
			"types":       {Uint32ToString(pgtype.Int4ArrayOID)},
			"values":      {"{1,2,3}"},
		},
		"SELECT array_int_column FROM public.test_table WHERE bool_column = FALSE": {
			"description": {"array_int_column"},
			"types":       {Uint32ToString(pgtype.Int4ArrayOID)},
			"values":      {""},
		},
		"SELECT array_ltree_column FROM public.test_table WHERE array_ltree_column IS NOT NULL": {
			"description": {"array_ltree_column"},
			"types":       {Uint32ToString(pgtype.TextArrayOID)},
			"values":      {"{a.b,c.d}"},
		},
		"SELECT array_ltree_column FROM public.test_table WHERE array_ltree_column IS NULL": {
			"description": {"array_ltree_column"},
			"types":       {Uint32ToString(pgtype.TextArrayOID)},
			"values":      {""},
		},
		"SELECT user_defined_column FROM public.test_table WHERE user_defined_column IS NOT NULL": {
			"description": {"user_defined_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"(Toronto)"},
		},
		"SELECT user_defined_column FROM public.test_table WHERE user_defined_column IS NULL": {
			"description": {"user_defined_column"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		// Typecasts
		"SELECT objoid, classoid, objsubid, description FROM pg_description WHERE classoid = 'pg_class'::regclass": {
			"description": {"objoid", "classoid", "objsubid", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.Int4OID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT d.objoid, d.classoid, c.relname, d.description FROM pg_description d JOIN pg_class c ON d.classoid = 'pg_class'::regclass": {
			"description": {"objoid", "classoid", "relname", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT objoid, classoid, objsubid, description FROM (SELECT * FROM pg_description WHERE classoid = 'pg_class'::regclass) d": {
			"description": {"objoid", "classoid", "objsubid", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.Int4OID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT objoid, classoid, objsubid, description FROM pg_description WHERE (classoid = 'pg_class'::regclass AND objsubid = 0) OR classoid = 'pg_type'::regclass": {
			"description": {"objoid", "classoid", "objsubid", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.Int4OID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT objoid, classoid, objsubid, description FROM pg_description WHERE classoid IN ('pg_class'::regclass, 'pg_type'::regclass)": {
			"description": {"objoid", "classoid", "objsubid", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.Int4OID), Uint32ToString(pgtype.TextOID)},
			"values":      {},
		},
		"SELECT objoid FROM pg_description WHERE classoid = CASE WHEN true THEN 'pg_class'::regclass ELSE 'pg_type'::regclass END": {
			"description": {"objoid"},
			"types":       {Uint32ToString(pgtype.OIDOID)},
			"values":      {},
		},
		"SELECT word FROM (VALUES ('abort', 'U', 't', 'unreserved', 'can be bare label')) t(word, catcode, barelabel, catdesc, baredesc) WHERE word <> ALL('{a,abs,absolute,action}'::text[])": {
			"description": {"word"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"abort"},
		},
		"SELECT t.x FROM (VALUES (1::int2, 'pg_type'::regclass)) t(x, y)": {
			"description": {"x"},
			"types":       {Uint32ToString(pgtype.Int2OID)},
			"values":      {"1"},
		},
		// SHOW
		"SHOW search_path": {
			"description": {"search_path"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {`"$user", public`},
		},
		// SELECT * FROM function()
		"SELECT * FROM pg_catalog.pg_get_keywords() LIMIT 1": {
			"description": {"word", "catcode", "barelabel", "catdesc", "baredesc"},
			"types":       {Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID)},
			"values":      {"abort", "U", "t", "unreserved", "can be bare label"},
		},
		"SELECT pg_get_keywords.word FROM pg_catalog.pg_get_keywords() LIMIT 1": {
			"description": {"word"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"abort"},
		},
		"SELECT * FROM generate_series(1, 2) AS series(index) LIMIT 1": {
			"description": {"index"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"1"},
		},
		"SELECT * FROM generate_series(1, array_upper(current_schemas(FALSE), 1)) AS series(index) LIMIT 1": {
			"description": {"index"},
			"types":       {Uint32ToString(pgtype.Int8OID)},
			"values":      {"1"},
		},
		// Transformed JOIN's
		"SELECT s.usename, r.rolconfig FROM pg_catalog.pg_shadow s LEFT JOIN pg_catalog.pg_roles r ON s.usename = r.rolname": {
			"description": {"usename", "rolconfig"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb", "NULL"},
		},
		"SELECT a.oid, pd.description FROM pg_catalog.pg_roles a LEFT JOIN pg_catalog.pg_shdescription pd ON a.oid = pd.objoid": {
			"description": {"oid", "description"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.BoolOID)},
			"values":      {"10", ""},
		},
		// CASE
		"SELECT CASE WHEN true THEN 'yes' ELSE 'no' END AS case": {
			"description": {"case"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"yes"},
		},
		"SELECT CASE WHEN false THEN 'yes' ELSE 'no' END AS case": {
			"description": {"case"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"no"},
		},
		"SELECT CASE WHEN true THEN 'one' WHEN false THEN 'two' ELSE 'three' END AS case": {
			"description": {"case"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"one"},
		},
		"SELECT CASE WHEN (SELECT count(extname) FROM pg_catalog.pg_extension WHERE extname = 'bdr') > 0 THEN 'pgd' WHEN (SELECT count(*) FROM pg_replication_slots) > 0 THEN 'log' ELSE NULL END AS type": {
			"description": {"type"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {""},
		},
		"SELECT roles.oid AS id, roles.rolname AS name, roles.rolsuper AS is_superuser, CASE WHEN roles.rolsuper THEN true ELSE false END AS can_create_role FROM pg_catalog.pg_roles roles WHERE rolname = current_user": {
			"description": {"id", "name", "is_superuser", "can_create_role"},
			"types":       {Uint32ToString(pgtype.Int8OID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT roles.oid AS id, roles.rolname AS name, roles.rolsuper AS is_superuser, CASE WHEN roles.rolsuper THEN true ELSE roles.rolcreaterole END AS can_create_role FROM pg_catalog.pg_roles roles WHERE rolname = current_user": {
			"description": {"id", "name", "is_superuser", "can_create_role"},
			"types":       {Uint32ToString(pgtype.Int8OID), Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT CASE WHEN TRUE THEN pg_catalog.pg_is_in_recovery() END AS CASE": {
			"description": {"case"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"f"},
		},
		// WHERE pg functions
		"SELECT gss_authenticated, encrypted FROM (SELECT false, false, false, false, false WHERE false) t(pid, gss_authenticated, principal, encrypted, credentials_delegated) WHERE pid = pg_backend_pid()": {
			"description": {"gss_authenticated", "encrypted"},
			"types":       {Uint32ToString(pgtype.BoolOID), Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		// WITH
		"WITH RECURSIVE simple_cte AS (SELECT oid, rolname FROM pg_roles WHERE rolname = 'postgres' UNION ALL SELECT oid, rolname FROM pg_roles) SELECT * FROM simple_cte": {
			"description": {"oid", "rolname"},
			"types":       {Uint32ToString(pgtype.OIDOID), Uint32ToString(pgtype.TextOID)},
			"values":      {"10", "bemidb"},
		},
		// Table alias
		"SELECT pg_shadow.usename FROM pg_shadow": {
			"description": {"usename"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb"},
		},
		"SELECT pg_roles.rolname FROM pg_roles": {
			"description": {"rolname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb"},
		},
		"SELECT pg_extension.extname FROM pg_extension": {
			"description": {"extname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"plpgsql"},
		},
		"SELECT pg_database.datname FROM pg_database": {
			"description": {"datname"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb"},
		},
		"SELECT pg_inherits.inhrelid FROM pg_inherits": {
			"description": {"inhrelid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT pg_shdescription.objoid FROM pg_shdescription": {
			"description": {"objoid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT pg_statio_user_tables.relid FROM pg_statio_user_tables": {
			"description": {"relid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT pg_replication_slots.slot_name FROM pg_replication_slots": {
			"description": {"slot_name"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT pg_stat_gssapi.pid FROM pg_stat_gssapi": {
			"description": {"pid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT pg_auth_members.oid FROM pg_auth_members": {
			"description": {"oid"},
			"types":       {Uint32ToString(pgtype.BoolOID)},
			"values":      {},
		},
		"SELECT tables.table_name FROM information_schema.tables": {
			"description": {"table_name"},
			"types":       {Uint32ToString(pgtype.TextOID)},
			"values":      {"test_table"},
		},
		// Sublink's in target list
		"SELECT x.usename, (SELECT passwd FROM pg_shadow WHERE usename = x.usename) as password FROM pg_shadow x WHERE x.usename = 'bemidb'": {
			"description": {"usename", "password"},
			"types":       {Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID)},
			"values":      {"bemidb", "bemidb-encrypted"},
		},
		// Type comparisons
		"SELECT db.oid AS did, db.datname AS name, ta.spcname AS spcname, db.datallowconn, db.datistemplate AS is_template, pg_catalog.has_database_privilege(db.oid, 'CREATE') AS cancreate, datdba AS owner, descr.description FROM pg_catalog.pg_database db LEFT OUTER JOIN pg_catalog.pg_tablespace ta ON db.dattablespace = ta.oid LEFT OUTER JOIN pg_catalog.pg_shdescription descr ON (db.oid = descr.objoid AND descr.classoid = 'pg_database'::regclass) WHERE db.oid > 1145::OID OR db.datname IN ('postgres', 'edb') ORDER BY datname": {
			"description": {"did", "name", "spcname", "datallowconn", "is_template", "cancreate", "owner", "description"},
			"types": {
				Uint32ToString(pgtype.OIDOID),
				Uint32ToString(pgtype.TextOID),
				Uint32ToString(pgtype.TextOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.BoolOID),
				Uint32ToString(pgtype.Int8OID),
				Uint32ToString(pgtype.BoolOID),
			},
			"values": {"16388", "bemidb", "", "true", "false", "true", "10", ""},
		},
	}

	for query, responses := range responsesByQuery {
		t.Run(query, func(t *testing.T) {
			queryHandler := initQueryHandler()

			messages, err := queryHandler.HandleQuery(query)

			testNoError(t, err)
			testRowDescription(t, messages[0], responses["description"], responses["types"])

			if len(responses["values"]) > 0 {
				testMessageTypes(t, messages, []pgproto3.Message{
					&pgproto3.RowDescription{},
					&pgproto3.DataRow{},
					&pgproto3.CommandComplete{},
				})
				testDataRowValues(t, messages[1], responses["values"])
			} else {
				testMessageTypes(t, messages, []pgproto3.Message{
					&pgproto3.RowDescription{},
					&pgproto3.CommandComplete{},
				})
			}

		})
	}

	t.Run("Returns an error if a table does not exist", func(t *testing.T) {
		queryHandler := initQueryHandler()

		_, err := queryHandler.HandleQuery("SELECT * FROM non_existent_table")

		if err == nil {
			t.Errorf("Expected an error, got nil")
		}

		expectedErrorMessage := strings.Join([]string{
			"Catalog Error: Table with name non_existent_table does not exist!",
			"Did you mean \"test_table\"?",
			"LINE 1: SELECT * FROM non_existent_table",
			"                      ^",
		}, "\n")
		if err.Error() != expectedErrorMessage {
			t.Errorf("Expected the error to be '"+expectedErrorMessage+"', got %v", err.Error())
		}
	})

	t.Run("Returns a result without a row description for SET queries", func(t *testing.T) {
		queryHandler := initQueryHandler()

		messages, err := queryHandler.HandleQuery("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL READ UNCOMMITTED")

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.CommandComplete{},
		})
	})
}

func TestHandleParseQuery(t *testing.T) {
	t.Run("Handles PARSE extended query", func(t *testing.T) {
		query := "SELECT usename, passwd FROM pg_shadow WHERE usename=$1"
		queryHandler := initQueryHandler()
		message := &pgproto3.Parse{Query: query}

		messages, preparedStatement, err := queryHandler.HandleParseQuery(message)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.ParseComplete{},
		})

		remappedQuery := "SELECT usename, passwd FROM (VALUES ('bemidb', '10'::int8, 'FALSE'::bool, 'FALSE'::bool, 'TRUE'::bool, 'FALSE'::bool, 'bemidb-encrypted', 'NULL', 'NULL')) pg_shadow(usename, usesysid, usecreatedb, usesuper, userepl, usebypassrls, passwd, valuntil, useconfig) WHERE usename = $1"
		if preparedStatement.Query != remappedQuery {
			t.Errorf("Expected the prepared statement query to be %v, got %v", remappedQuery, preparedStatement.Query)
		}
		if preparedStatement.Statement == nil {
			t.Errorf("Expected the prepared statement to have a statement")
		}
	})
}

func TestHandleBindQuery(t *testing.T) {
	t.Run("Handles BIND extended query with text format parameter", func(t *testing.T) {
		queryHandler := initQueryHandler()
		query := "SELECT usename, passwd FROM pg_shadow WHERE usename=$1"
		parseMessage := &pgproto3.Parse{Query: query}
		_, preparedStatement, err := queryHandler.HandleParseQuery(parseMessage)
		testNoError(t, err)

		bindMessage := &pgproto3.Bind{
			Parameters:           [][]byte{[]byte("bemidb")},
			ParameterFormatCodes: []int16{0}, // Text format
		}
		messages, preparedStatement, err := queryHandler.HandleBindQuery(bindMessage, preparedStatement)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.BindComplete{},
		})
		if len(preparedStatement.Variables) != 1 {
			t.Errorf("Expected the prepared statement to have 1 variable, got %v", len(preparedStatement.Variables))
		}
		if preparedStatement.Variables[0] != "bemidb" {
			t.Errorf("Expected the prepared statement variable to be 'bemidb', got %v", preparedStatement.Variables[0])
		}
	})

	t.Run("Handles BIND extended query with binary format parameter", func(t *testing.T) {
		queryHandler := initQueryHandler()
		query := "SELECT c.oid FROM pg_catalog.pg_class c WHERE c.relnamespace = $1"
		parseMessage := &pgproto3.Parse{Query: query}
		_, preparedStatement, err := queryHandler.HandleParseQuery(parseMessage)
		testNoError(t, err)

		paramValue := int64(2200)
		paramBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(paramBytes, uint64(paramValue))

		bindMessage := &pgproto3.Bind{
			Parameters:           [][]byte{paramBytes},
			ParameterFormatCodes: []int16{1}, // Binary format
		}
		messages, preparedStatement, err := queryHandler.HandleBindQuery(bindMessage, preparedStatement)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.BindComplete{},
		})
		if len(preparedStatement.Variables) != 1 {
			t.Errorf("Expected the prepared statement to have 1 variable, got %v", len(preparedStatement.Variables))
		}
		if preparedStatement.Variables[0] != paramValue {
			t.Errorf("Expected the prepared statement variable to be %v, got %v", paramValue, preparedStatement.Variables[0])
		}
	})
}

func TestHandleDescribeQuery(t *testing.T) {
	t.Run("Handles DESCRIBE extended query", func(t *testing.T) {
		queryHandler := initQueryHandler()
		query := "SELECT usename, passwd FROM pg_shadow WHERE usename=$1"
		parseMessage := &pgproto3.Parse{Query: query}
		messages, preparedStatement, err := queryHandler.HandleParseQuery(parseMessage)
		bindMessage := &pgproto3.Bind{Parameters: [][]byte{[]byte("bemidb")}}
		messages, preparedStatement, err = queryHandler.HandleBindQuery(bindMessage, preparedStatement)
		message := &pgproto3.Describe{ObjectType: 'P'}

		messages, preparedStatement, err = queryHandler.HandleDescribeQuery(message, preparedStatement)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.RowDescription{},
		})
		testRowDescription(t, messages[0], []string{"usename", "passwd"}, []string{Uint32ToString(pgtype.TextOID), Uint32ToString(pgtype.TextOID)})
		if preparedStatement.Rows == nil {
			t.Errorf("Expected the prepared statement to have rows")
		}
	})
}

func TestHandleExecuteQuery(t *testing.T) {
	t.Run("Handles EXECUTE extended query", func(t *testing.T) {
		queryHandler := initQueryHandler()
		query := "SELECT usename, passwd FROM pg_shadow WHERE usename=$1"
		parseMessage := &pgproto3.Parse{Query: query}
		messages, preparedStatement, err := queryHandler.HandleParseQuery(parseMessage)
		bindMessage := &pgproto3.Bind{Parameters: [][]byte{[]byte("bemidb")}}
		messages, preparedStatement, err = queryHandler.HandleBindQuery(bindMessage, preparedStatement)
		describeMessage := &pgproto3.Describe{ObjectType: 'P'}
		messages, preparedStatement, err = queryHandler.HandleDescribeQuery(describeMessage, preparedStatement)
		message := &pgproto3.Execute{}

		messages, err = queryHandler.HandleExecuteQuery(message, preparedStatement)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.DataRow{},
			&pgproto3.CommandComplete{},
		})
		testDataRowValues(t, messages[0], []string{"bemidb", "bemidb-encrypted"})
	})
}

func TestHandleMultipleQueries(t *testing.T) {
	t.Run("Handles multiple SET statements", func(t *testing.T) {
		query := `SET client_encoding TO 'UTF8';
SET client_min_messages TO 'warning';
SET standard_conforming_strings = on;`
		queryHandler := initQueryHandler()

		messages, err := queryHandler.HandleQuery(query)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.CommandComplete{},
		})
	})

	t.Run("Handles mixed SET and SELECT statements", func(t *testing.T) {
		query := `SET client_encoding TO 'UTF8';
SELECT passwd FROM pg_shadow WHERE usename='bemidb';`
		queryHandler := initQueryHandler()

		messages, err := queryHandler.HandleQuery(query)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.RowDescription{},
			&pgproto3.DataRow{},
			&pgproto3.CommandComplete{},
		})
		testDataRowValues(t, messages[1], []string{"bemidb-encrypted"})
	})

	t.Run("Handles multiple SELECT statements", func(t *testing.T) {
		query := `SELECT passwd FROM pg_shadow WHERE usename='bemidb';
SELECT passwd FROM pg_shadow WHERE usename='bemidb';`
		queryHandler := initQueryHandler()

		messages, err := queryHandler.HandleQuery(query)

		testNoError(t, err)
		testMessageTypes(t, messages, []pgproto3.Message{
			&pgproto3.RowDescription{},
			&pgproto3.DataRow{},
			&pgproto3.CommandComplete{},
		})
		testDataRowValues(t, messages[1], []string{"bemidb-encrypted"})
	})

	t.Run("Handles error in any of multiple statements", func(t *testing.T) {
		query := `SET client_encoding TO 'UTF8';
SELECT * FROM non_existent_table;
SET standard_conforming_strings = on;`
		queryHandler := initQueryHandler()

		_, err := queryHandler.HandleQuery(query)

		if err == nil {
			t.Error("Expected an error for non-existent table, got nil")
			return
		}

		if !strings.Contains(err.Error(), "non_existent_table") {
			t.Errorf("Expected error message to contain 'non_existent_table', got: %s", err.Error())
		}
	})
}

func initQueryHandler() *QueryHandler {
	config := loadTestConfig()
	duckdb := NewDuckdb(config)
	icebergReader := NewIcebergReader(config)
	return NewQueryHandler(config, duckdb, icebergReader)
}

func testNoError(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func testMessageTypes(t *testing.T, messages []pgproto3.Message, expectedTypes []pgproto3.Message) {
	if len(messages) != len(expectedTypes) {
		t.Errorf("Expected %v messages, got %v", len(expectedTypes), len(messages))
	}

	for i, expectedType := range expectedTypes {
		if reflect.TypeOf(messages[i]) != reflect.TypeOf(expectedType) {
			t.Errorf("Expected the %v message to be a %v", i, expectedType)
		}
	}
}

func testRowDescription(t *testing.T, rowDescriptionMessage pgproto3.Message, expectedColumnNames []string, expectedColumnTypes []string) {
	rowDescription := rowDescriptionMessage.(*pgproto3.RowDescription)

	if len(rowDescription.Fields) != len(expectedColumnNames) {
		t.Errorf("Expected %v row description fields, got %v", len(expectedColumnNames), len(rowDescription.Fields))
	}

	for i, expectedColumnName := range expectedColumnNames {
		if string(rowDescription.Fields[i].Name) != expectedColumnName {
			t.Errorf("Expected the %v row description field to be %v, got %v", i, expectedColumnName, string(rowDescription.Fields[i].Name))
		}
	}

	for i, expectedColumnType := range expectedColumnTypes {
		if Uint32ToString(rowDescription.Fields[i].DataTypeOID) != expectedColumnType {
			t.Errorf("Expected the %v row description field data type to be %v, got %v", i, expectedColumnType, Uint32ToString(rowDescription.Fields[i].DataTypeOID))
		}
	}
}

func testDataRowValues(t *testing.T, dataRowMessage pgproto3.Message, expectedValues []string) {
	dataRow := dataRowMessage.(*pgproto3.DataRow)

	if len(dataRow.Values) != len(expectedValues) {
		t.Errorf("Expected %v data row values, got %v", len(expectedValues), len(dataRow.Values))
	}

	for i, expectedValue := range expectedValues {
		if string(dataRow.Values[i]) != expectedValue {
			t.Errorf("Expected the %v data row value to be %v, got %v", i, expectedValue, string(dataRow.Values[i]))
		}
	}
}
