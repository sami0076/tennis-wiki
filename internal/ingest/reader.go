package ingest

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound means the source has no file for that season, which is normal:
// mirrors differ in coverage.
var ErrNotFound = errors.New("source file not found")

// Fetcher opens a season's file for a source.
type Fetcher interface {
	Open(ctx context.Context, s Source, season int) (io.ReadCloser, error)
}

// PathFetcher opens a file by its path under a base. Reference data -- player
// tables and ranking history -- is not seasonal, so it addresses files this way
// rather than through Fetcher.
type PathFetcher interface {
	OpenPath(ctx context.Context, baseURL, relPath string) (io.ReadCloser, error)
}

// LocalFetcher reads from a clone on disk. Tests use it exclusively, so no test
// touches the network.
type LocalFetcher struct{ Root string }

// Open returns the season's file from under Root.
func (l LocalFetcher) Open(ctx context.Context, s Source, season int) (io.ReadCloser, error) {
	return l.OpenPath(ctx, s.BaseURL, s.RelPath(season))
}

// OpenPath returns a file from under Root, ignoring the base URL.
func (l LocalFetcher) OpenPath(_ context.Context, _, relPath string) (io.ReadCloser, error) {
	path := filepath.Join(l.Root, filepath.FromSlash(relPath))
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		// Fall back to the bare filename: a local clone may be flat where the
		// mirror nests, and vice versa.
		flat := filepath.Join(l.Root, filepath.Base(relPath))
		if f2, err2 := os.Open(flat); err2 == nil {
			return f2, nil
		}
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, nil
}

// HTTPFetcher reads from a mirror over HTTPS.
type HTTPFetcher struct {
	Client *http.Client
}

// Open requests the season's file from the source's mirror.
func (h HTTPFetcher) Open(ctx context.Context, s Source, season int) (io.ReadCloser, error) {
	return h.OpenPath(ctx, s.BaseURL, s.RelPath(season))
}

// OpenPath requests one file from a mirror.
func (h HTTPFetcher) OpenPath(ctx context.Context, baseURL, relPath string) (io.ReadCloser, error) {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	url := strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(relPath, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		// errors.Join drops a nil close error, so this stays one path.
		closeErr := resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.Join(fmt.Errorf("%w: %s", ErrNotFound, url), closeErr)
		}
		return nil, errors.Join(
			fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status), closeErr)
	}
	return resp.Body, nil
}

// Reader streams MatchRows from one source file.
type Reader struct {
	csv  *csv.Reader
	cols *columns
	src  Source
	rows int
}

// NewReader reads the header and prepares to stream rows.
func NewReader(src Source, r io.Reader) (*Reader, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	// Row widths vary across eras within a single family.
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header of %s: %w", src.Name, err)
	}
	cols, err := newColumns(src.Profile, header)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src.Name, err)
	}
	return &Reader{csv: cr, cols: cols, src: src}, nil
}

// Next returns the next row. It returns io.EOF when the file is exhausted.
func (r *Reader) Next() (MatchRow, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return MatchRow{}, err
	}
	r.rows++
	row, err := parseRow(r.cols, rec)
	if err != nil {
		return MatchRow{}, fmt.Errorf("%s row %d: %w", r.src.Name, r.rows, err)
	}
	return row, nil
}

// Rows returns how many records have been read, header excluded.
func (r *Reader) Rows() int { return r.rows }
