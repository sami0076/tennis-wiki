package name

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Novak Djokovic":     "novak-djokovic",
		"Novak Djoković":     "novak-djokovic",
		"Jo-Wilfried Tsonga": "jo-wilfried-tsonga",
		"Alex De Minaur":     "alex-de-minaur",
		"Karolína Plíšková":  "karolina-pliskova",
		"Björn Borg":         "bjorn-borg",
		"  padded  name  ":   "padded-name",
		"O'Connell":          "o-connell",
		"":                   "",
		"???":                "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

// The whole point: a query typed with diacritics has to fold to what the
// sources actually store, which is plain ASCII.
func TestNormaliseFoldsQueriesToStoredForm(t *testing.T) {
	cases := map[string]string{
		"Djoković":          "djokovic",
		"djokovic":          "djokovic",
		"Karolína Plíšková": "karolina pliskova",
		"  Federer ":        "federer",
		"Muñoz":             "munoz",
		"Ćorić":             "coric",
	}
	for in, want := range cases {
		if got := Normalise(in); got != want {
			t.Errorf("Normalise(%q) = %q, want %q", in, got, want)
		}
	}
}

// Slug and Normalise must fold identically, or a name found by search would not
// resolve to the profile URL built from the same name.
func TestSlugAndNormaliseAgree(t *testing.T) {
	for _, in := range []string{"Novak Djoković", "Karolína Plíšková", "Björn Borg", "Ćorić"} {
		slug := Slug(in)
		normalised := Normalise(in)
		if slug != replaceAll(normalised, ' ', '-') {
			t.Errorf("%q: slug %q and normalised %q disagree", in, slug, normalised)
		}
	}
}

func replaceAll(s string, from, to byte) string {
	b := []byte(s)
	for i := range b {
		if b[i] == from {
			b[i] = to
		}
	}
	return string(b)
}

// A CJK name has no ASCII equivalent to fold to; dropping the characters must
// not leave a run of separators behind.
func TestUnfoldableCharactersAreDropped(t *testing.T) {
	if got := Slug("Kei 錦織 Nishikori"); got != "kei-nishikori" {
		t.Errorf("Slug = %q, want kei-nishikori", got)
	}
}
