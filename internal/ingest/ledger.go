package ingest

import (
	"context"
	"fmt"
	"strconv"
)

// FileKey identifies one ingestible file. Unit is the season for matches and
// the file's path for reference data, which is not seasonal.
type FileKey struct {
	Source string
	Unit   string
}

// SeasonKey names the file a source holds for one season.
func SeasonKey(source string, season int) FileKey {
	return FileKey{Source: source, Unit: strconv.Itoa(season)}
}

// PathKey names a reference file, which is addressed by path.
func PathKey(source, relPath string) FileKey {
	return FileKey{Source: source, Unit: relPath}
}

// Ledger remembers what each source file looked like when it was last read, so
// an unchanged one can be skipped rather than re-read and re-upserted.
type Ledger interface {
	// IngestedFiles returns the validator recorded for every file read so far.
	IngestedFiles(ctx context.Context) (map[FileKey]string, error)
	// RecordFile is called once a file's rows are all committed.
	RecordFile(ctx context.Context, k FileKey, validator string, seen, written int) error
}

// IngestedFiles returns the validator recorded for every file read so far.
func (s *Store) IngestedFiles(ctx context.Context) (map[FileKey]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT source, unit, validator FROM ingest_files`)
	if err != nil {
		return nil, fmt.Errorf("read ingest ledger: %w", err)
	}
	defer rows.Close()
	out := map[FileKey]string{}
	for rows.Next() {
		var k FileKey
		var validator string
		if err := rows.Scan(&k.Source, &k.Unit, &validator); err != nil {
			return nil, err
		}
		out[k] = validator
	}
	return out, rows.Err()
}

// RecordFile marks a file ingested at the given validator. It is written only
// after every row of that file has been committed, so a recorded file is one
// the database genuinely holds.
func (s *Store) RecordFile(ctx context.Context, k FileKey, validator string, seen, written int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO ingest_files (source, unit, validator, rows_seen, rows_written)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source, unit) DO UPDATE
		   SET validator = EXCLUDED.validator, ingested_at = now(),
		       rows_seen = EXCLUDED.rows_seen, rows_written = EXCLUDED.rows_written`,
		k.Source, k.Unit, validator, seen, written)
	if err != nil {
		return fmt.Errorf("record ingested file %s/%s: %w", k.Source, k.Unit, err)
	}
	return nil
}
