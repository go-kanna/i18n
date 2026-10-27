package gen

import (
	"errors"
	"fmt"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/output"
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

// Analyze analyzes the locale files locales/<tag>.yaml, whose sources locales
// holds by tag, with def as the default language. declared are the names the
// destination package declares, each as name@file:line. It spells each
// constructor as its name and its parameters, such as
// "ItemsCount(count int, price number)", the catalogs as their tags in order,
// and each diagnostic as diag.Diag.String does.
func Analyze(def string, locales map[string]string, declared ...string) (constructors, catalogs, diags []string) {
	var (
		cs  = make([]catalog, 0, len(locales))
		dc  catalog
		dds []diag.Diag
	)
	for _, tag := range slices.Sorted(maps.Keys(locales)) {
		path := "locales/" + tag + ".yaml"
		entries, ds := parseLocale(path, []byte(locales[tag]))
		dds = append(dds, ds...)
		c := catalog{tag: language.MustParse(tag), path: path, entries: entries}
		if tag == def {
			dc = c
		}
		cs = append(cs, c)
	}
	names := make(map[string]token.Position, len(declared))
	for _, d := range declared {
		name, at, _ := strings.Cut(d, "@")
		file, line, _ := strings.Cut(at, ":")
		n, _ := strconv.Atoi(line)
		names[name] = token.Position{Filename: "messages/" + file, Line: n}
	}

	m, ds := analyze(dc, cs, names)
	for _, msg := range m.messages {
		params := make([]string, len(msg.params))
		for i, p := range msg.params {
			params[i] = goName(p.name) + " " + p.kind.String()
		}
		constructors = append(constructors, msg.fn+"("+strings.Join(params, ", ")+")")
	}
	for _, c := range m.catalogs {
		catalogs = append(catalogs, c.tag.String())
	}
	for _, d := range append(dds, ds...) {
		diags = append(diags, d.String())
	}
	return constructors, catalogs, diags
}

// Emit generates the package messages from the locale files in dir/locales,
// with en the default language, keeping clear of what the files already in
// dir/messages declare.
func Emit(dir string) ([]byte, error) {
	cs, ds := readLocales(filepath.Join(dir, "locales"))
	def, dds := defaultCatalog(filepath.Join(dir, "locales"), cs, language.English)
	declared := output.DeclaredNames(filepath.Join(dir, "messages"), "i18n_gen.go", "messages")
	m, ads := analyze(def, cs, declared)
	if all := slices.Concat(ds, dds, ads); diag.HasErrors(all) {
		msgs := make([]string, len(all))
		for i, d := range all {
			msgs[i] = d.String()
		}
		return nil, errors.New(strings.Join(msgs, "\n"))
	}
	src, err := emit(m, "messages", slices.Sorted(maps.Keys(declared))).Bytes()
	if err != nil {
		return src, fmt.Errorf("generated code does not parse: %w", err)
	}
	return src, nil
}
