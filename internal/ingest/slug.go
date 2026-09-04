package ingest

import (
	"fmt"

	"github.com/sami0076/tennis-wiki/internal/name"
)

// DisambiguateSlug produces the nth alternative for a slug that is already
// taken. The sequence is stable, so the same input always yields the same slug
// and bookmarked URLs survive a re-ingest.
func DisambiguateSlug(base string, tour Tour, attempt int) string {
	switch attempt {
	case 0:
		return base
	case 1:
		return fmt.Sprintf("%s-%s", base, tour)
	default:
		return fmt.Sprintf("%s-%s-%d", base, tour, attempt-1)
	}
}

// Slugify is name.Slug. The folding is shared with search, which has to agree
// with it: see the package comment there.
func Slugify(s string) string { return name.Slug(s) }
