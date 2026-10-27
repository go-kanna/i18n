package gen

import (
	"fmt"
	"go/token"
	"path/filepath"
	"strconv"

	"github.com/go-kanna/internal/diag"
	"golang.org/x/text/language"
)

// ParseTemplate parses src, spelling each segment as its text quoted or its
// placeholder in braces, each parameter as name:kind with a ! after a kind
// written out, and each diagnostic as diag.Diag.String does.
func ParseTemplate(src string) (segments, params, diags []string) {
	t, ds := parseTemplate(src, token.Position{})
	for _, s := range t.segments {
		if s.param != "" {
			segments = append(segments, "{"+s.param+"}")
			continue
		}
		segments = append(segments, strconv.Quote(s.text))
	}
	for _, p := range t.params {
		spelled := p.name + ":" + p.kind.String()
		if p.explicit {
			spelled += "!"
		}
		params = append(params, spelled)
	}
	for _, d := range ds {
		diags = append(diags, d.String())
	}
	return segments, params, diags
}

// ParseLocale reads src as the locale file en.yaml, spelling each message as
// its key and where it is written, then each form: its category, if any, its
// text quoted, and where it is written, such as
// `items@1:1 one "One"@3:10 other "Many"@4:12`; and each diagnostic as
// diag.Diag.String does.
func ParseLocale(src string) (entries, diags []string) {
	es, ds := parseLocale("en.yaml", []byte(src))
	for _, e := range es {
		spelled := fmt.Appendf(nil, "%s@%d:%d", e.key, e.pos.Line, e.pos.Column)
		for _, f := range e.forms {
			if f.category != "" {
				spelled = fmt.Appendf(spelled, " %s", f.category)
			}
			spelled = fmt.Appendf(spelled, " %q@%d:%d", f.src, f.pos.Line, f.pos.Column)
		}
		entries = append(entries, string(spelled))
	}
	for _, d := range ds {
		diags = append(diags, d.String())
	}
	return entries, diags
}

// ReadLocales reads the locale directory dir/locales, spelling each catalog as
// its language, its file name, and how many messages it holds, such as
// "en en.yaml 2"; and each diagnostic as diag.Diag.String does, its file
// relative to dir.
func ReadLocales(dir string) (catalogs, diags []string) {
	cs, ds := readLocales(filepath.Join(dir, "locales"))
	for _, c := range cs {
		catalogs = append(catalogs, fmt.Sprintf("%s %s %d", c.tag, filepath.Base(c.path), len(c.entries)))
	}
	for _, d := range ds {
		d.Pos.Filename = diag.ShortPath(dir, d.Pos.Filename)
		diags = append(diags, d.String())
	}
	return catalogs, diags
}

// DefaultCatalog picks lang as the default language among the locale files
// locales/<tag>.yaml of tags, returning the language picked, and each
// diagnostic as diag.Diag.String spells it.
func DefaultCatalog(lang string, tags ...string) (string, []string) {
	cs := make([]catalog, len(tags))
	for i, t := range tags {
		cs[i] = catalog{tag: language.MustParse(t), path: "locales/" + t + ".yaml"}
	}
	c, ds := defaultCatalog("locales", cs, language.MustParse(lang))
	diags := make([]string, 0, len(ds))
	for _, d := range ds {
		diags = append(diags, d.String())
	}
	return c.tag.String(), diags
}
