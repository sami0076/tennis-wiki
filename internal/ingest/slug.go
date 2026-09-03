package ingest

import (
	"fmt"
	"strings"
	"unicode"
)

// Slugify turns a player name into a URL key: lowercase ASCII words joined by
// hyphens. Diacritics are folded so "Novak Djoković" and "Novak Djokovic"
// produce the same slug and therefore collide, which is the intent — the
// collision is then resolved deterministically by DisambiguateSlug.
func Slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	lastHyphen := true // suppresses a leading hyphen

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastHyphen = false
		case fold(r) != "":
			b.WriteString(fold(r))
			lastHyphen = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// A letter with no ASCII equivalent, such as a CJK character. Drop
			// it rather than emit a hyphen run.
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

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

// fold maps the accented Latin letters that appear in player names to ASCII.
// Chosen over a full Unicode normalisation dependency because the domain is
// narrow and the mapping needs to be explicit and testable.
func fold(r rune) string {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ā', 'ă', 'ą':
		return "a"
	case 'æ':
		return "ae"
	case 'ç', 'ć', 'č', 'ĉ':
		return "c"
	case 'ď', 'đ':
		return "d"
	case 'é', 'è', 'ê', 'ë', 'ē', 'ĕ', 'ė', 'ę', 'ě':
		return "e"
	case 'ğ', 'ģ':
		return "g"
	case 'ħ':
		return "h"
	case 'í', 'ì', 'î', 'ï', 'ī', 'į', 'ı':
		return "i"
	case 'ĺ', 'ľ', 'ł':
		return "l"
	case 'ñ', 'ń', 'ň':
		return "n"
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'ø', 'ō', 'ő':
		return "o"
	case 'œ':
		return "oe"
	case 'ŕ', 'ř':
		return "r"
	case 'ś', 'š', 'ş', 'ș':
		return "s"
	case 'ß':
		return "ss"
	case 'ť', 'ţ', 'ț':
		return "t"
	case 'ú', 'ù', 'û', 'ü', 'ū', 'ů', 'ű', 'ų':
		return "u"
	case 'ý', 'ÿ':
		return "y"
	case 'ź', 'ž', 'ż':
		return "z"
	default:
		return ""
	}
}
