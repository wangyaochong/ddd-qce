// Package pg provides PostgreSQL utilities: PgTransactionManager with nested
// transaction support via savepoints, Migrate/DropAll/TruncateAll for schema
// management, and helper functions like NullString/NullTime/JSONOrNull.
//
// Use IsUniqueViolation(err) to check for PostgreSQL unique constraint violations.
package pg
