package gen_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-kanna/i18n/internal/gen"
)

// writeFiles writes files, by path relative to dir, a path ending in a slash
// being an empty directory.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, body := range files {
		path := filepath.Join(dir, name)
		if name[len(name)-1] == '/' {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReadLocales(t *testing.T) {
	t.Parallel()

	const msg = "greeting: \"Hi\"\n"

	tests := []struct {
		name         string
		files        map[string]string
		wantCatalogs []string
		wantDiags    []string
	}{
		{
			name: "one file per language, in the order of their names",
			files: map[string]string{
				"locales/pt-BR.yaml": msg,
				"locales/en.yaml":    msg + "bye: \"Bye\"\n",
				"locales/ja.yml":     msg,
			},
			wantCatalogs: []string{"en en.yaml 2", "ja ja.yml 1", "pt-BR pt-BR.yaml 1"},
		},
		{
			name:         "tags and extensions spelled otherwise",
			files:        map[string]string{"locales/en_us.YAML": msg},
			wantCatalogs: []string{"en-US en_us.YAML 1"},
		},
		{
			name:         "an empty file",
			files:        map[string]string{"locales/en.yaml": ""},
			wantCatalogs: []string{"en en.yaml 0"},
		},
		{
			name: "names starting with a dot",
			files: map[string]string{
				"locales/en.yaml":     msg,
				"locales/.DS_Store":   "",
				"locales/.ja.yaml":    msg,
				"locales/.git/x.yaml": msg,
			},
			wantCatalogs: []string{"en en.yaml 1"},
		},
		{
			name: "other files",
			files: map[string]string{
				"locales/en.yaml":     msg,
				"locales/config.yaml": "key: value\n",
				"locales/README":      "",
				"locales/notes.txt":   "",
				"locales/root.yaml":   msg,
			},
			wantCatalogs: []string{"en en.yaml 1"},
			wantDiags: []string{
				`locales/README: warning: skipped: "README" is not a language tag`,
				`locales/config.yaml: warning: skipped: "config" is not a language tag`,
				`locales/notes.txt: warning: skipped: "notes" is not a language tag`,
				`locales/root.yaml: warning: skipped: "root" is not a language tag`,
			},
		},
		{
			name: "directories",
			files: map[string]string{
				"locales/en.yaml":               msg,
				"locales/ja/messages.yaml":      msg,
				"locales/fr.yml/":               "",
				"locales/translations/de.yaml":  msg,
				"locales/scripts/convert.sh":    "",
				"locales/empty/":                "",
				"locales/translations/nested/":  "",
				"locales/scripts/data/de.yaml/": "",
			},
			wantCatalogs: []string{"en en.yaml 1"},
			wantDiags: []string{
				"locales/fr.yml: error: a language is one file, not a directory\n  merge the files in it into fr.yaml",
				"locales/ja: error: a language is one file, not a directory\n  merge the files in it into ja.yaml",
				"locales/translations: warning: skipped: locale files are read from the locale directory alone\n" +
					"  move them into locales",
			},
		},
		{
			name: "named like a locale file, but not YAML",
			files: map[string]string{
				"locales/en.yaml": msg,
				"locales/ja.json": "{}",
				"locales/de.toml": "",
				"locales/fr":      "",
			},
			wantCatalogs: []string{"en en.yaml 1"},
			wantDiags: []string{
				"locales/de.toml: error: named like a locale file, but not YAML\n  a locale file ends in .yaml or .yml",
				"locales/fr: error: named like a locale file, but not YAML\n  a locale file ends in .yaml or .yml",
				"locales/ja.json: error: named like a locale file, but not YAML\n  a locale file ends in .yaml or .yml",
			},
		},
		{
			name: "two files for a language",
			files: map[string]string{
				"locales/en.yaml":    msg,
				"locales/en.yml":     msg,
				"locales/pt-BR.yaml": msg,
				"locales/pt_br.yaml": msg,
			},
			wantCatalogs: []string{"en en.yaml 1", "pt-BR pt-BR.yaml 1"},
			wantDiags: []string{
				"locales/en.yml: error: a second locale file for en, after en.yaml\n  keep one file per language",
				"locales/pt_br.yaml: error: a second locale file for pt-BR, after pt-BR.yaml\n  keep one file per language",
			},
		},
		{
			name: "mistakes in every file",
			files: map[string]string{
				"locales/en.yaml": "Greeting: \"Hi\"\n",
				"locales/ja.yaml": "greeting: 1\n",
			},
			wantCatalogs: []string{"en en.yaml 0", "ja ja.yaml 0"},
			wantDiags: []string{
				`locales/en.yaml:1:1: error: invalid key "Greeting": each part of a key matches [a-z][a-z0-9_]*`,
				"locales/ja.yaml:1:11: error: message of key \"greeting\" is not a string\n" +
					"  quote it to have it read as text: \"1\"",
			},
		},
		{
			name:  "no locale files",
			files: map[string]string{"locales/config.yaml": ""},
			wantDiags: []string{
				`locales/config.yaml: warning: skipped: "config" is not a language tag`,
				"./locales: error: no locale files\n  name each YAML file after its language, such as en.yaml",
			},
		},
		{
			name:  "no locale directory",
			files: map[string]string{"other/en.yaml": msg},
			wantDiags: []string{
				"./locales: error: cannot read the locale directory: no such file or directory\n" +
					"  -locales names the directory holding the locale files",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			catalogs, diags := gen.ReadLocales(dir)
			if !slices.Equal(catalogs, tt.wantCatalogs) {
				t.Errorf("catalogs = %q, want %q", catalogs, tt.wantCatalogs)
			}
			if !slices.Equal(diags, tt.wantDiags) {
				t.Errorf("diagnostics =\n%q\nwant\n%q", diags, tt.wantDiags)
			}
		})
	}
}

// A linked directory named like a language is a directory, not a file to read.
func TestReadLocales_linkedDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"locales/en.yaml": "", "shared/de/messages.yaml": ""})
	if err := os.Symlink(filepath.Join(dir, "shared", "de"), filepath.Join(dir, "locales", "de")); err != nil {
		t.Fatal(err)
	}

	_, diags := gen.ReadLocales(dir)
	want := []string{"locales/de: error: a language is one file, not a directory\n  merge the files in it into de.yaml"}
	if !slices.Equal(diags, want) {
		t.Errorf("diagnostics =\n%q\nwant\n%q", diags, want)
	}
}

// A locale file that cannot be read is reported, and the others are read.
func TestReadLocales_unreadable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"locales/en.yaml": ""})
	if err := os.Symlink(filepath.Join(dir, "gone.yaml"), filepath.Join(dir, "locales", "ja.yaml")); err != nil {
		t.Fatal(err)
	}

	catalogs, diags := gen.ReadLocales(dir)
	if want := []string{"en en.yaml 0"}; !slices.Equal(catalogs, want) {
		t.Errorf("catalogs = %q, want %q", catalogs, want)
	}
	if want := []string{"locales/ja.yaml: error: cannot read: no such file or directory"}; !slices.Equal(diags, want) {
		t.Errorf("diagnostics =\n%q\nwant\n%q", diags, want)
	}
}

func TestDefaultCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lang      string
		tags      []string
		want      string
		wantDiags []string
	}{
		{name: "itself", lang: "en", tags: []string{"en-GB", "en", "ja"}, want: "en"},
		{name: "a region of it", lang: "en", tags: []string{"ja", "en-US"}, want: "en-US"},
		{name: "a script of it", lang: "zh", tags: []string{"zh-Hant"}, want: "zh-Hant"},
		// A default written with a region is served by its own file alone.
		{
			name: "the language of a region",
			lang: "en-US",
			tags: []string{"ja", "en"},
			wantDiags: []string{
				"locales: error: no locale file for en-US, the default language\n" +
					"  there are locale files for ja, en; choose one with -default",
			},
		},
		{
			name: "another region of the language",
			lang: "en-US",
			tags: []string{"en-GB"},
			wantDiags: []string{
				"locales: error: no locale file for en-US, the default language\n" +
					"  there are locale files for en-GB; choose one with -default",
			},
		},
		{
			name: "one of two regions",
			lang: "en",
			tags: []string{"en-GB", "en-US"},
			want: "en-US",
			wantDiags: []string{
				"locales: warning: the default language en could be en-GB or en-US; taking en-US\n" +
					"  pass -default en-US to say which",
			},
		},
		{
			name: "one of three regions",
			lang: "en",
			tags: []string{"en-AU", "en-GB", "en-US"},
			want: "en-US",
			wantDiags: []string{
				"locales: warning: the default language en could be en-AU, en-GB, or en-US; taking en-US\n" +
					"  pass -default en-US to say which",
			},
		},
		{
			name: "none",
			lang: "en",
			tags: []string{"de", "ja"},
			wantDiags: []string{
				"locales: error: no locale file for en, the default language\n" +
					"  there are locale files for de, ja; choose one with -default",
			},
		},
		// CLDR matches Norwegian Bokmål with Danish, which would make Danish
		// write every constructor.
		{
			name: "none, however close",
			lang: "nb",
			tags: []string{"da"},
			wantDiags: []string{
				"locales: error: no locale file for nb, the default language\n" +
					"  there are locale files for da; choose one with -default",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := gen.DefaultCatalog(tt.lang, tt.tags...)
			if !slices.Equal(diags, tt.wantDiags) {
				t.Errorf("diagnostics =\n%q\nwant\n%q", diags, tt.wantDiags)
			}
			if len(tt.wantDiags) == 0 || tt.want != "" {
				if got != tt.want {
					t.Errorf("DefaultCatalog() = %s, want %s", got, tt.want)
				}
			}
		})
	}
}
