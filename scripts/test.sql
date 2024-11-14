-- Usage: psql postgres://127.0.0.1:5432/dbname -P pager=off -f ./scripts/test.sql

DROP TABLE IF EXISTS test_table;
DROP TYPE IF EXISTS address;

CREATE TYPE address AS (
  city VARCHAR(50)
);

CREATE TABLE test_table (
  id SERIAL PRIMARY KEY,
  bool_column BOOLEAN,
  bpchar_column BPCHAR(10),
  varchar_column VARCHAR(255),
  text_column TEXT,
  int2_column INT2,
  int4_column INT4,
  int8_column INT8,
  float4_column FLOAT4,
  float8_column FLOAT8,
  numeric_column NUMERIC(10, 2),
  date_column DATE,
  time_column TIME,
  time_ms_column TIME(3),
  timetz_column TIMETZ,
  timetz_ms_column TIMETZ(3),
  timestamp_column TIMESTAMP,
  timestamp_ms_column TIMESTAMP(3),
  timestamptz_column TIMESTAMPTZ,
  timestamptz_ms_column TIMESTAMPTZ(3),
  uuid_column UUID,
  bytea_column BYTEA,
  interval_column INTERVAL,
  json_column JSON,
  jsonb_column JSONB,
  tsvector_column TSVECTOR,
  array_text_column TEXT[],
  array_int_column INT[],
  user_defined_column address
);

INSERT INTO test_table (
  bool_column,
  bpchar_column,
  varchar_column,
  text_column,
  int2_column,
  int4_column,
  int8_column,
  float4_column,
  float8_column,
  numeric_column,
  date_column,
  time_column,
  time_ms_column,
  timetz_column,
  timetz_ms_column,
  timestamp_column,
  timestamp_ms_column,
  timestamptz_column,
  timestamptz_ms_column,
  uuid_column,
  bytea_column,
  interval_column,
  json_column,
  jsonb_column,
  tsvector_column,
  array_text_column,
  array_int_column,
  user_defined_column
) VALUES (
  TRUE,
  'bpchar',
  'varchar',
  'text',
  32767::INT2,
  2147483647::INT4,
  9223372036854775807::INT8,
  3.14::FLOAT4,
  3.141592653589793::FLOAT8,
  12345.67::NUMERIC(10, 2),
  '2024-01-01',
  '12:00:00.123456',
  '12:00:00.123',
  '12:00:00.123456-05',
  '12:00:00.123-05',
  '2024-01-01 12:00:00.123456',
  '2024-01-01 12:00:00.123',
  '2024-01-01 12:00:00.123456-05',
  '2024-01-01 12:00:00.123-05',
  gen_random_uuid(),
  decode('48656c6c6f', 'hex'),
  '1 mon 2 days 01:00:01.000001'::INTERVAL,
  '{"key": "value"}'::JSON,
  '{"key": "value"}'::JSONB,
  to_tsvector('Sample text for tsvector'),
  '{"one", "two", "three"}',
  '{1, 2, 3}',
  ROW('Toronto')
), (
  FALSE,
  '',
  NULL,
  '',
  -32767::INT2,
  NULL,
  -9223372036854775807::INT8,
  NULL,
  -3.141592653589793::FLOAT8,
  -12345.00::NUMERIC(10, 2),
  NULL,
  '12:00:00.123',
  NULL,
  '12:00:00.12300+05',
  '12:00:00.1+05',
  '2024-01-01 12:00:00',
  NULL,
  '2024-01-01 12:00:00.000123+05',
  '2024-01-01 12:00:00.12+05',
  NULL,
  NULL,
  NULL,
  NULL,
  '{}'::JSONB,
  NULL,
  NULL,
  '{}',
  NULL
);

SELECT
  table_schema,
  table_name,
  column_name,
  data_type,
  udt_name,
  is_nullable,
  character_maximum_length,
  numeric_precision,
  numeric_scale,
  datetime_precision
FROM information_schema.columns
WHERE table_schema NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
ORDER BY table_schema, table_name, ordinal_position;