package gen

import (
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/output"
	"github.com/go-kanna/internal/packages"
	"golang.org/x/mod/modfile"
)

// callers loads the code that calls the messages: the destination's own, and
// that of the packages of its module importing it, tests included, with the
// output of an earlier run set aside where it no longer compiles. It reports
// false when there is nothing to load, as when no module holds the destination
// or no Go code is there yet.
//
// A syntax error in the destination stops the run, as the names it declares
// cannot all be told. Any other error of the load is what the code was like
// before the run, which checked holds what the run changes against.
func callers(dest string) (packages.Set, bool, []diag.Diag) {
	destPath, err := importPath(dest)
	if err != nil {
		return packages.Set{}, false, []diag.Diag{diag.Errorf(token.Position{Filename: dest}, "%v", err)}
	}
	if destPath == "" {
		return packages.Set{}, false, nil
	}
	root := moduleRoot(dest)

	patterns, err := packages.Dependents(root, destPath)
	if err != nil {
		return packages.Set{}, false, []diag.Diag{diag.Errorf(token.Position{}, "%v", err)}
	}
	if names, _ := filepath.Glob(filepath.Join(dest, "*.go")); len(names) > 0 {
		patterns = append(patterns, destPath)
	}
	if len(patterns) == 0 {
		return packages.Set{}, false, nil
	}
	// The output imports the runtime, which nothing loaded may yet.
	patterns = append(patterns, runtimePath)

	set, err := packages.Load(patterns, packages.Config{Dir: root, Tests: true, Regenerated: []string{outputFile}})
	if err != nil {
		return packages.Set{}, false, []diag.Diag{diag.Errorf(token.Position{}, "%v", err)}
	}
	blocking, _ := set.Split()
	if stop := slices.DeleteFunc(blocking, func(d diag.Diag) bool { return !in(dest, d) }); diag.HasErrors(stop) {
		return set, false, append(stop, set.SetAsideWarnings()...)
	}
	return set, true, nil
}

// importPath returns the import path of dest, which need not exist yet, in the
// module that holds it, or "" for none.
func importPath(dest string) (string, error) {
	root := moduleRoot(dest)
	if root == "" {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // the go.mod of the destination
	if err != nil {
		return "", err //nolint:wrapcheck // a *fs.PathError, which names the file
	}
	rel, err := filepath.Rel(root, dest)
	if err != nil {
		return "", err //nolint:wrapcheck // it names both paths
	}
	return path.Join(modfile.ModulePath(data), filepath.ToSlash(rel)), nil
}

// checked type-checks again the code callers loaded, with changes in place, and
// returns what is wrong with it then: every error left in the destination,
// which the output joins and whose errors keep its callers from being checked,
// and elsewhere those the load did not report already.
func checked(set packages.Set, dest string, changes []output.Change) []diag.Diag {
	rds := packages.Recheck(set, changes)
	return slices.Concat(
		slices.DeleteFunc(slices.Clone(rds), func(d diag.Diag) bool { return !in(dest, d) }),
		set.Introduced(slices.DeleteFunc(slices.Clone(rds), func(d diag.Diag) bool { return in(dest, d) })),
	)
}

// in reports whether d is about a file in dir.
func in(dir string, d diag.Diag) bool {
	return d.Pos.Filename != "" && output.SameDir(dir, filepath.Dir(d.Pos.Filename))
}

// moduleRoot returns the directory of the go.mod that holds dir, which need not
// exist yet, or "" for none.
func moduleRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
