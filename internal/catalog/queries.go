package catalog

// Every query here reads pg_catalog, information_schema or a statistics view.
// None of them touches a user relation: reading data starts in the validation
// layer, in a later phase. Keeping that boundary explicit is what lets the tool
// state that nothing can leak from this phase, because nothing is read.

// queryRelations lists the tables in scope.
//
// relispartition excludes child partitions: a partitioned table is read from
// the parent, because iterating partitions would double-count rows and produce
// statistics that match nothing the user recognizes.
const queryRelations = `
SELECT n.nspname,
       c.relname,
       c.relkind = 'p'                                            AS partitioned,
       EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid = c.oid
                 AND c.relkind = 'r')                             AS inherits,
       c.reltuples::bigint                                        AS estimated_rows,
       pg_total_relation_size(c.oid)                              AS total_bytes,
       coalesce(obj_description(c.oid, 'pg_class'), '')           AS comment,
       has_table_privilege(c.oid, 'SELECT')                       AS readable
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE c.relkind IN ('r', 'p')
   AND NOT c.relispartition
   AND n.nspname = ANY($1)
 ORDER BY n.nspname, c.relname`

// querySchemas lists the schemas a scope can be resolved from.
//
// The USAGE check is what keeps a schema the role cannot open out of scope
// entirely. Without it every table inside would land in the "no SELECT
// privilege" list, which presents as a finding what is merely the absence of any
// relationship between this role and that part of the database. Not being able
// to read a table inside a schema you can open is evidence of an incomplete
// grant; not being able to open the schema is scope.
const querySchemas = `
SELECT n.nspname
  FROM pg_namespace n
 WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND n.nspname NOT LIKE 'pg_toast%'
   AND n.nspname NOT LIKE 'pg_temp%'
   AND has_schema_privilege(n.oid, 'USAGE')
 ORDER BY n.nspname`

// queryColumns reads attributes with both the formatted type and the base type
// name. Comparing formatted types directly produces false negatives between
// equivalent spellings of the same type, which is why both are kept.
const queryColumns = `
SELECT n.nspname,
       c.relname,
       a.attname,
       format_type(a.atttypid, a.atttypmod)                       AS formatted_type,
       t.typname                                                  AS base_type,
       NOT a.attnotnull                                           AS nullable,
       coalesce(pg_get_expr(d.adbin, d.adrelid), '')              AS default_expr,
       a.attnum::int                                              AS position,
       coalesce(col_description(c.oid, a.attnum), '')             AS comment
  FROM pg_attribute a
  JOIN pg_class c     ON c.oid = a.attrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_type t      ON t.oid = a.atttypid
  LEFT JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
 WHERE a.attnum > 0
   AND NOT a.attisdropped
   AND c.relkind IN ('r', 'p')
   AND NOT c.relispartition
   AND n.nspname = ANY($1)
 ORDER BY n.nspname, c.relname, a.attnum`

// queryConstraints reads primary keys, uniques and foreign keys in one pass.
//
// convalidated is the field that matters most here. A constraint created NOT
// VALID and never validated blocks new violations but never checked the rows
// already there, while looking identical to any other in \d — dropping the
// field on read would erase the evidence behind an entire finding.
//
// Column numbers are resolved to names in SQL rather than in Go, so the
// ordering inside a composite key comes straight from the catalog.
const queryConstraints = `
SELECT n.nspname,
       c.relname,
       con.conname,
       con.contype::text,
       con.convalidated,
       coalesce(fn.nspname, '')                                   AS ref_schema,
       coalesce(fc.relname, '')                                   AS ref_table,
       (SELECT array_agg(a.attname ORDER BY k.ord)
          FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = con.conrelid
                             AND a.attnum = k.attnum)             AS columns,
       (SELECT array_agg(a.attname ORDER BY k.ord)
          FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = con.confrelid
                             AND a.attnum = k.attnum)             AS ref_columns
  FROM pg_constraint con
  JOIN pg_class c     ON c.oid = con.conrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
  LEFT JOIN pg_class fc     ON fc.oid = con.confrelid
  LEFT JOIN pg_namespace fn ON fn.oid = fc.relnamespace
 WHERE con.contype IN ('p', 'u', 'f')
   AND c.relkind IN ('r', 'p')
   AND NOT c.relispartition
   AND n.nspname = ANY($1)
 ORDER BY n.nspname, c.relname, con.conname`

// queryIndexes reads index columns in order.
//
// An expression element has attnum 0 and no matching attribute, so the LEFT
// JOIN yields an empty name. That placeholder is kept rather than dropped:
// removing it would shift a later column into leading position and make an
// unusable index look usable.
const queryIndexes = `
SELECT n.nspname,
       c.relname,
       ic.relname                                                 AS index_name,
       i.indisunique,
       i.indisprimary,
       (SELECT array_agg(coalesce(a.attname, '') ORDER BY k.ord)
          FROM unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord)
          LEFT JOIN pg_attribute a ON a.attrelid = i.indrelid
                                  AND a.attnum = k.attnum
                                  AND k.attnum <> 0)              AS columns
  FROM pg_index i
  JOIN pg_class c      ON c.oid = i.indrelid
  JOIN pg_class ic     ON ic.oid = i.indexrelid
  JOIN pg_namespace n  ON n.oid = c.relnamespace
 WHERE c.relkind IN ('r', 'p')
   AND NOT c.relispartition
   AND n.nspname = ANY($1)
 ORDER BY n.nspname, c.relname, ic.relname`

// queryUsageStats reads activity counters. They are meaningless without the
// reset moment, which queryStatsReset supplies alongside.
const queryUsageStats = `
SELECT schemaname,
       relname,
       coalesce(seq_scan, 0),
       coalesce(idx_scan, 0),
       coalesce(n_tup_ins, 0),
       coalesce(n_tup_upd, 0),
       coalesce(n_tup_del, 0)
  FROM pg_stat_user_tables
 WHERE schemaname = ANY($1)`

// queryStatsReset reads when the counters started counting. A null result means
// they were never reset, or that the moment cannot be determined; either way the
// counters must be treated as uninterpretable.
const queryStatsReset = `
SELECT stats_reset
  FROM pg_stat_database
 WHERE datname = current_database()`
