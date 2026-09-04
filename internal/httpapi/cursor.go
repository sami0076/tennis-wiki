package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// Cursor pagination, never offset. On a table the size of match_players an
// OFFSET has to walk every skipped row, and any insert between two pages shifts
// the window so a client sees a row twice or not at all. A cursor names the last
// row seen, so both problems go away.

// ErrBadCursor means the token did not come from this API, or came from an
// older ordering.
var ErrBadCursor = errors.New("malformed cursor")

// EncodeCursor packs the ordering key of the last row on a page. It is opaque on
// purpose: the key can change without breaking a documented URL contract.
func EncodeCursor(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// DecodeCursor reverses EncodeCursor into v.
func DecodeCursor(s string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ErrBadCursor
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return ErrBadCursor
	}
	return nil
}

// Page is the envelope for every list response. NextCursor is empty on the last
// page, which is how a client knows to stop.
type Page[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// Limit reads a bounded page size from the query string.
func Limit(r *http.Request, def, max int) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be a number, got %q", raw)
	}
	if n < 1 || n > max {
		return 0, fmt.Errorf("limit must be between 1 and %d, got %d", max, n)
	}
	return n, nil
}
