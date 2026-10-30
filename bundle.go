package i18n

import (
	"fmt"
	"slices"

	"golang.org/x/text/language"
)

// Bundle holds the catalogs of every language a package of messages was
// generated from. It never changes once built, so it and the Localizers it
// hands out can be used from any number of goroutines.
//
// The zero Bundle has no languages, and its Localizers render every message as
// its key.
type Bundle struct {
	matcher language.Matcher
	// chains holds, for each language in the order the matcher knows them,
	// where a message is looked up.
	chains [][]locale
}

// NewBundle builds a Bundle from def, the catalog of the default language, and
// rest, those of the other languages. It copies what the catalogs hold, so a
// change to one afterwards changes nothing the Bundle renders.
//
// It panics on a language that is not a valid tag and on two catalogs of one
// language. Generated code passes neither, so meeting one means a Bundle put
// together by hand, and failing at once shows the mistake where it was made.
func NewBundle(def Catalog, rest ...Catalog) Bundle {
	locales := make([]locale, 0, 1+len(rest))
	byTag := make(map[language.Tag]locale, 1+len(rest))
	for _, c := range append([]Catalog{def}, rest...) {
		l, err := newLocale(c)
		if err != nil {
			panic("i18n: " + err.Error())
		}
		if _, ok := byTag[l.tag]; ok {
			panic(fmt.Sprintf("i18n: two catalogs of %s", l.tag))
		}
		byTag[l.tag] = l
		locales = append(locales, l)
	}

	tags := make([]language.Tag, len(locales))
	chains := make([][]locale, len(locales))
	for i, l := range locales {
		tags[i] = l.tag
		chains[i] = chain(l, byTag, locales[0])
	}
	// The matcher falls back to the first tag it is given, the default.
	return Bundle{matcher: language.NewMatcher(tags), chains: chains}
}

// chain lists where a message of l is looked up: l itself, then each of its
// parent languages with a catalog (en-GB, then en), then the default language.
// The built-in texts of a language follow the last of its catalogs, so that a
// catalog overrides them, and a reader of Japanese is shown the built-in
// Japanese before the default language's own text.
func chain(l locale, byTag map[language.Tag]locale, def locale) []locale {
	c := []locale{l}
	for t := l.tag.Parent(); t != language.Und; t = t.Parent() {
		if p, ok := byTag[t]; ok {
			c = append(c, p)
		}
	}
	if !slices.ContainsFunc(c, func(x locale) bool { return x.tag == def.tag }) {
		c = append(c, def)
	}

	var out []locale
	for i, x := range c {
		out = append(out, x)
		base, _ := x.tag.Base()
		if b, ok := builtinLocales[base]; ok &&
			!slices.ContainsFunc(c[i+1:], func(y locale) bool { yb, _ := y.tag.Base(); return yb == base }) {
			out = append(out, b)
		}
	}
	return out
}

// builtinLocales are the catalogs of builtin, by base language.
var builtinLocales = func() map[language.Base]locale {
	m := make(map[language.Base]locale, len(builtin))
	for _, c := range builtin {
		l, _ := newLocale(c) // the languages of builtin are valid tags
		base, _ := l.tag.Base()
		m[base] = l
	}
	return m
}()

// Localizer returns a Localizer for the language of b closest to langs.
//
// Each of langs is a language tag, such as "ja" or "pt-BR", or an
// Accept-Language value, such as "fr, ja;q=0.8". They are tried in order until
// one is close to a language of b, as CLDR judges closeness: en-US is served
// by en, and nb by da when b has Danish but no Norwegian. So a language the
// user picked can come before the ones the browser asks for. A value that does
// not parse is skipped as a whole, one bad entry and all, and when nothing
// matches, the default language is used.
func (b Bundle) Localizer(langs ...string) Localizer {
	if b.matcher == nil {
		return Localizer{}
	}
	_, i := language.MatchStrings(b.matcher, langs...)
	return Localizer{chain: b.chains[i]}
}

// Localizer renders messages in the language a Bundle chose for it. The zero
// Localizer renders every message as its key.
type Localizer struct {
	chain []locale
}

// Localize renders m. A message missing from the language of l is looked up in
// its parent languages and then in the default language, and is rendered under
// the plural rules and number format of the language it is found in.
//
// Localize never fails: a message no language has renders as its key.
func (l Localizer) Localize(m Message) string {
	for _, loc := range l.chain {
		if e, ok := loc.entries[m.Key]; ok {
			return loc.render(e, m.Args)
		}
	}
	return m.Key
}
