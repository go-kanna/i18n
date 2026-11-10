package i18n

import (
	"fmt"
	"maps"
	"slices"
	"strings"

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
// user picked can come before the ones the browser asks for. An entry of a
// value that does not parse is skipped, and the rest of the value is still
// tried. When nothing matches, the default language is used.
func (b Bundle) Localizer(langs ...string) Localizer {
	if b.matcher == nil {
		return Localizer{}
	}
	for _, s := range langs {
		desired := parseLanguages(s)
		if len(desired) == 0 {
			continue
		}
		if _, i, conf := b.matcher.Match(desired...); conf != language.No {
			return Localizer{chain: b.chains[i]}
		}
	}
	_, i, _ := b.matcher.Match()
	return Localizer{chain: b.chains[i]}
}

// parseLanguages returns the languages of s, an Accept-Language value, by
// weight, leaving out the entries that do not parse. The weight is read in
// either case, as Q=0.5, the way RFC 9110 has it.
func parseLanguages(s string) []language.Tag {
	var entries []string
	for e := range strings.SplitSeq(s, ",") {
		if tag, params, ok := strings.Cut(e, ";"); ok {
			e = tag + ";" + strings.ToLower(params)
		}
		if _, _, err := language.ParseAcceptLanguage(e); err == nil {
			entries = append(entries, e)
		}
	}
	tags, _, _ := language.ParseAcceptLanguage(strings.Join(entries, ","))
	return tags
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
	if loc, e, ok := find(l.chain, m.Key); ok {
		return loc.render(e, m.Args)
	}
	return m.Key
}

func lookupEntries(b Bundle, key string) []Entry {
	var out []Entry
	for _, c := range b.chains {
		if _, e, ok := find(c, key); ok {
			out = append(out, cloneEntry(e))
		}
	}
	return out
}

func bundleKeys(b Bundle) []string {
	keys := make(map[string]bool)
	for _, c := range b.chains {
		for _, loc := range c {
			for key := range loc.entries {
				keys[key] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(keys))
}

// find returns the first entry of chain keyed key, and the language it is in.
func find(chain []locale, key string) (locale, Entry, bool) {
	for _, loc := range chain {
		if e, ok := loc.entries[key]; ok {
			return loc, e, true
		}
	}
	return locale{}, Entry{}, false
}
