// Package ingest reads match, player, and ranking data from the configured
// sources and writes it into the database.
//
// Ingestion is streaming and idempotent: re-running over the same input
// produces no duplicate rows and no changed rows. Sources are declared in
// configuration rather than hardcoded, because the project's original
// upstream repositories were removed mid-build. See ADR-0002.
package ingest
