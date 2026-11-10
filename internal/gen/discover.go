package gen

import (
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-kanna/internal/diag"
	"golang.org/x/text/language"
)

// catalog is a locale file read: the messages of one language.
type catalog struct {
	tag     language.Tag
	path    string
	entries []entry
}

// yamlExts are the extensions of a locale file.
var yamlExts = []string{".yaml", ".yml"}

// readLocales reads the locale files in dir, one per language, in the order of
// their names.
//
// The directory is flat, and nothing named like a language is skipped in
// silence: a directory named like a language or a locale file, or a file named
// so that is not YAML, is an error, since its language would otherwise drop out
// of the generated package without a word. Anything else is skipped with a
// warning, except a directory that holds no YAML and a name starting with a
// dot, which are skipped without one.
func readLocales(dir string) ([]catalog, []diag.Diag) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		d := diag.Errorf(token.Position{Filename: dir}, "cannot read the locale directory: %v", pathless(err))
		return nil, []diag.Diag{d.WithHints("-locales names the directory holding the locale files")}
	}

	var (
		catalogs []catalog
		diags    []diag.Diag
	)
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		pos := token.Position{Filename: path}
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)

		if isDir(path) {
			if _, ok := languageOf(stem); ok {
				diags = append(diags, diag.Errorf(pos, "a language is one file, not a directory").
					WithHints(fmt.Sprintf("merge the files in it into %s.yaml", stem)))
			} else if holdsYAML(path) {
				diags = append(diags, diag.Warningf(pos, "skipped: locale files are read from the locale directory alone").
					WithHints("move them into "+filepath.Base(dir)))
			}
			continue
		}

		tag, ok := languageOf(stem)
		switch {
		case !ok:
			diags = append(diags, diag.Warningf(pos, "skipped: %q is not a language tag", stem))
			continue
		case !slices.Contains(yamlExts, strings.ToLower(ext)):
			diags = append(diags, diag.Errorf(pos, "named like a locale file, but not YAML").
				WithHints("a locale file ends in .yaml or .yml"))
			continue
		}
		if i := slices.IndexFunc(catalogs, func(c catalog) bool { return c.tag == tag }); i >= 0 {
			first := filepath.Base(catalogs[i].path)
			diags = append(diags, diag.Errorf(pos, "a second locale file for %s, after %s", tag, first).
				WithHints("keep one file per language"))
			continue
		}
		if !hasCLDRData(tag) {
			diags = append(diags, diag.Warningf(pos,
				"x/text holds no CLDR data for %s, so it has no plural rules or number format of its own", tag).
				WithHints(fmt.Sprintf("check that %q names a language; keep other files out of %s", stem, filepath.Base(dir))))
		}

		data, err := os.ReadFile(path) //nolint:gosec // reading the locale files the user points at is the point
		if err != nil {
			diags = append(diags, diag.Errorf(pos, "cannot read: %v", pathless(err)))
			continue
		}
		entries, ds := parseLocale(path, data)
		diags = append(diags, ds...)
		catalogs = append(catalogs, catalog{tag: tag, path: path, entries: entries})
	}

	if len(catalogs) == 0 && !diag.HasErrors(diags) {
		diags = append(diags, diag.Errorf(token.Position{Filename: dir}, "no locale files").
			WithHints("name each YAML file after its language, such as en.yaml"))
	}
	return catalogs, diags
}

// languageOf returns the language that s, a file name without its extension,
// names. A tag without a language, such as und or x-foo, names none.
func languageOf(s string) (language.Tag, bool) {
	tag, err := language.Parse(s)
	if err != nil {
		return language.Und, false
	}
	_, ok := baseOf(tag)
	return tag, ok
}

// hasCLDRData reports whether x/text holds CLDR data for the language of tag.
// A three-letter name such as app or dev reads as an ISO 639-3 language, which
// is likely no language the file is meant for.
func hasCLDRData(tag language.Tag) bool {
	base, _ := tag.Base()
	_, exact := language.CompactIndex(language.Make(base.String()))
	return exact
}

// baseOf returns the language of tag, without its region or script. A tag
// that does not write one has none, although x/text would guess one.
func baseOf(tag language.Tag) (language.Base, bool) {
	base, conf := tag.Base()
	return base, conf == language.Exact
}

// defaultCatalog picks the catalog of lang, the default language, among
// catalogs read from dir: the one of lang exactly. Another catalog of the
// language never stands in for it, en-US.yaml for en nor en.yaml for en-US:
// adding the catalog of lang would then change what every constructor says
// without a word. A catalog of another language is never taken, however close.
func defaultCatalog(dir string, catalogs []catalog, lang language.Tag) (catalog, []diag.Diag) {
	if i := slices.IndexFunc(catalogs, func(c catalog) bool { return c.tag == lang }); i >= 0 {
		return catalogs[i], nil
	}

	base, ok := baseOf(lang)
	have := make([]string, len(catalogs))
	var same []language.Tag
	for i, c := range catalogs {
		have[i] = c.tag.String()
		if b, _ := baseOf(c.tag); ok && b == base {
			same = append(same, c.tag)
		}
	}
	d := diag.Errorf(token.Position{Filename: dir}, "no locale file for %s, the default language", lang)
	if len(same) == 1 {
		return catalog{}, []diag.Diag{d.WithHints(fmt.Sprintf("did you mean -default %s?", same[0]))}
	}
	return catalog{}, []diag.Diag{d.WithHints(
		fmt.Sprintf("there are locale files for %s; choose one with -default", strings.Join(have, ", ")))}
}

// isDir reports whether path is a directory, following a symbolic link, so
// that a linked directory named like a language is not taken for a file.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// holdsYAML reports whether the directory at path holds a YAML file directly.
func holdsYAML(path string) bool {
	ents, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(ents, func(e fs.DirEntry) bool {
		return !e.IsDir() && slices.Contains(yamlExts, strings.ToLower(filepath.Ext(e.Name())))
	})
}

// pathless drops the path from err, which a diagnostic puts in its position.
func pathless(err error) error {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return pe.Err
	}
	return err
}
