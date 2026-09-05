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

// ErrUnchanged means the file is byte-for-byte what it was when the caller's
// validator was issued, so there is nothing to read.
var ErrUnchanged = errors.New("source file unchanged")

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

// ConditionalFetcher opens a file only when it differs from what the validator
// describes, and returns a validator for what it did open. An empty validator
// asks for the file unconditionally.
//
// Both stages address files by path in the end, so one method serves matches
// and reference data alike.
type ConditionalFetcher interface {
	OpenPathIfChanged(ctx context.Context, baseURL, relPath, validator string) (
		body io.ReadCloser, current string, err error)
}

// LocalFetcher reads from a clone on disk. Tests use it exclusively, so no test
// touches the network.
type LocalFetcher struct{ Root string }

// Open returns the season's file from under Root.
func (l LocalFetcher) Open(ctx context.Context, s Source, season int) (io.ReadCloser, error) {
	return l.OpenPath(ctx, s.BaseURL, s.RelPath(season))
}

// OpenPath returns a file from under Root, ignoring the base URL.
func (l LocalFetcher) OpenPath(ctx context.Context, baseURL, relPath string) (io.ReadCloser, error) {
	body, _, err := l.OpenPathIfChanged(ctx, baseURL, relPath, "")
	return body, err
}

// OpenPathIfChanged compares size and modification time, which is what a local
// clone offers in place of an ETag.
func (l LocalFetcher) OpenPathIfChanged(_ context.Context, _, relPath, validator string) (
	io.ReadCloser, string, error) {
	path := filepath.Join(l.Root, filepath.FromSlash(relPath))
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// Fall back to the bare filename: a local clone may be flat where the
		// mirror nests, and vice versa.
		flat := filepath.Join(l.Root, filepath.Base(relPath))
		if _, err2 := os.Stat(flat); err2 == nil {
			path = flat
		} else {
			return nil, "", fmt.Errorf("%w: %s", ErrNotFound, path)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("stat %s: %w", path, err)
	}
	current := fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano())
	if validator != "" && validator == current {
		return nil, current, ErrUnchanged
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	return f, current, nil
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
	body, _, err := h.OpenPathIfChanged(ctx, baseURL, relPath, "")
	return body, err
}

// OpenPathIfChanged sends the validator as If-None-Match. The mirrors answer an
// unchanged file with 304 and no body, which is the whole point: a full run
// re-reads a few hundred files, and transferring them again is most of its time.
func (h HTTPFetcher) OpenPathIfChanged(ctx context.Context, baseURL, relPath, validator string) (
	io.ReadCloser, string, error) {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	url := strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(relPath, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request for %s: %w", url, err)
	}
	if validator != "" {
		req.Header.Set("If-None-Match", validator)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		// errors.Join drops a nil close error, so this stays one path.
		closeErr := resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusNotModified:
			return nil, validator, errors.Join(ErrUnchanged, closeErr)
		case http.StatusNotFound:
			return nil, "", errors.Join(fmt.Errorf("%w: %s", ErrNotFound, url), closeErr)
		}
		return nil, "", errors.Join(
			fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status), closeErr)
	}
	return resp.Body, resp.Header.Get("ETag"), nil
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
