package gen

import (
	"path/filepath"

	"github.com/go-kanna/internal/deps"
	"github.com/go-kanna/internal/output"
)

// reportDeps tells what a run in dir reads and writes. The code calling the
// messages is checked by a run but not read by it, so that is left out, and the
// destination is there for what the code in it imports.
func reportDeps(dir string, cfg config, _ []string) (deps.Report, error) {
	locales, dest := absolute(dir, cfg.locales), absolute(dir, cfg.destination)
	r := deps.Report{
		Inputs: []string{locales},
		Writes: []string{filepath.Join(dest, outputFile)},
	}

	destPath, err := importPath(dest)
	if err != nil || destPath == "" {
		return r, nil //nolint:nilerr // the run reports a destination no module holds
	}
	if names, _ := filepath.Glob(filepath.Join(dest, "*.go")); len(names) > 0 {
		r.Packages = []string{destPath}
	}

	catalogs, _ := readLocales(locales)
	def, _ := defaultCatalog(locales, catalogs, cfg.def)
	pkg, err := output.PackageName(dest, outputFile, cfg.pkg)
	if err != nil {
		return r, nil //nolint:nilerr // which the run reports
	}
	messages, _ := constructors(def, output.DeclaredNames(dest, outputFile, pkg))
	r.Declares = []string{destPath + "." + localizerFunc}
	for _, m := range messages {
		r.Declares = append(r.Declares, destPath+"."+m.fn)
	}
	return r, nil
}
