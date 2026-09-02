// Package db holds the pgx connection pool setup and the sqlc-generated
// typed queries.
//
// There is no ORM here by design: the statistical work leans on PostgreSQL
// window functions, and sqlc gives type safety without hiding the SQL. See
// ADR-0004.
package db
