package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type searchCursor struct {
	Rank float32 `json:"r"`
	ID   int64   `json:"i"`
}

func TestCursorRoundTrip(t *testing.T) {
	want := searchCursor{Rank: 0.85, ID: 12345}
	token, err := EncodeCursor(want)
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	// It travels in a query string, so it must survive without escaping.
	if strings.ContainsAny(token, "+/=&?#") {
		t.Errorf("cursor %q is not URL-safe", token)
	}

	var got searchCursor
	if err := DecodeCursor(token, &got); err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if got != want {
		t.Errorf("round trip gave %+v, want %+v", got, want)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"not base64!!", "", "d2hhdGV2ZXI"} {
		var got searchCursor
		if err := DecodeCursor(bad, &got); !errors.Is(err, ErrBadCursor) {
			t.Errorf("DecodeCursor(%q) = %v, want ErrBadCursor", bad, err)
		}
	}
}

func TestLimit(t *testing.T) {
	cases := []struct {
		query   string
		want    int
		wantErr bool
	}{
		{"", 25, false},
		{"?limit=10", 10, false},
		{"?limit=100", 100, false},
		{"?limit=0", 0, true},
		{"?limit=101", 0, true},
		{"?limit=-5", 0, true},
		{"?limit=abc", 0, true},
	}
	for _, c := range cases {
		t.Run("limit"+c.query, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/players"+c.query, nil)
			got, err := Limit(r, 25, 100)
			if c.wantErr {
				if err == nil {
					t.Errorf("Limit(%q) = %d, want an error", c.query, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Limit(%q): %v", c.query, err)
			}
			if got != c.want {
				t.Errorf("Limit(%q) = %d, want %d", c.query, got, c.want)
			}
		})
	}
}
