// Package name normalises player names for URL slugs and for search.
//
// Both uses need the same folding, and they have to agree: the stored names
// come from sources that already write plain ASCII, so a visitor searching for
// "Djoković" only finds "Novak Djokovic" if the query is folded the same way
// the slug was.
package name

import (
	"strings"
	"unicode"
)

// Slug turns a player name into a URL key: lowercase ASCII words joined by
// hyphens. Diacritics are folded so "Novak Djoković" and "Novak Djokovic"
// produce the same slug and therefore collide, which is the intent — the
// collision is then resolved deterministically by the caller.
func Slug(s string) string { return convert(s, '-') }

// Normalise folds a search query to the same ASCII the stored names use,
// keeping spaces so trigram similarity still sees word boundaries.
func Normalise(s string) string { return convert(s, ' ') }

func convert(s string, sep byte) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSep := true // suppresses a leading separator

	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastSep = false
		case fold(r) != "":
			b.WriteString(fold(r))
			lastSep = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// A letter with no ASCII equivalent, such as a CJK character. Drop
			// it rather than emit a run of separators.
		case !lastSep:
			b.WriteByte(sep)
			lastSep = true
		}
	}
	return strings.Trim(b.String(), string(sep))
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
