package gen

import (
	"flag"
	"go/token"
	"maps"
	"path/filepath"
	"slices"

	"github.com/go-kanna/internal/cli"
	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/output"
	"golang.org/x/text/language"
)

// outputFile is the file kanna-i18n writes into the destination.
const outputFile = "i18n_gen.go"

// config holds what kanna-i18n's own flags set.
type config struct {
	locales     string
	def         language.Tag
	destination string
	pkg         string
}

// Command returns kanna-i18n.
func Command() cli.Command[config] {
	return cli.Command[config]{
		Name:    generator,
		Summary: "compile a directory of locale files into typed message constructors",
		Help: "Each locale file is YAML named by its language, such as en.yaml or pt-BR.yaml.\n" +
			"The default language defines the constructors, and every other language is\n" +
			"checked against it.",
		Flags: func(fs *flag.FlagSet, cfg *config) {
			fs.StringVar(&cfg.locales, "locales", "locales", "read the locale files from `dir`")
			fs.TextVar(&cfg.def, "default", language.English,
				"take `lang` as the default language, which defines the constructors and is the last fallback")
			fs.StringVar(&cfg.destination, "destination", "messages", "write "+outputFile+" into `dir`")
			fs.StringVar(&cfg.pkg, "package", "",
				"call the package `name` when the destination has no Go files yet (default: the directory's name)")
		},
		Run:  run,
		Deps: reportDeps,
	}
}

func run(dir string, cfg config, _ []string) ([]output.Change, []diag.Diag) {
	locales, dest := absolute(dir, cfg.locales), absolute(dir, cfg.destination)

	catalogs, diags := readLocales(locales)
	if diag.HasErrors(diags) {
		return nil, diags
	}
	def, ds := defaultCatalog(locales, catalogs, cfg.def)
	if diags = append(diags, ds...); diag.HasErrors(diags) {
		return nil, diags
	}

	// The code calling the messages is loaded before the run writes them, as it
	// is, to be checked once they are in place.
	set, check, ds := callers(dest)
	if diags = append(diags, ds...); diag.HasErrors(diags) {
		return nil, diags
	}

	pkg, err := output.PackageName(dest, outputFile, cfg.pkg)
	if err != nil {
		return nil, append(diags, diag.Errorf(token.Position{Filename: dest}, "%v", err))
	}
	declared := output.DeclaredNames(dest, outputFile, pkg)

	m, ds := analyze(def, catalogs, declared)
	if diags = append(diags, ds...); diag.HasErrors(diags) {
		return nil, diags
	}

	path := filepath.Join(dest, outputFile)
	f := emit(m, pkg, slices.Sorted(maps.Keys(declared)))
	src, err := f.Bytes()
	if err != nil {
		return nil, append(diags, diag.Errorf(token.Position{Filename: path}, "generated code does not parse: %v", err))
	}
	diags = append(diags, output.MissingRequires(path, f.ModuleImports())...)
	changes := []output.Change{{Path: path, Data: src}}
	if check {
		if rds := checked(set, dest, changes); len(rds) > 0 {
			return nil, slices.Concat(diags, rds, set.SetAsideWarnings())
		}
	}
	return changes, diags
}

// absolute returns path, given on the command line, as an absolute path, taking
// a relative one to start from dir.
func absolute(dir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(dir, path)
}
