// Package vet is kanna-i18n vet, which finds the codes a program reports that
// the messages kanna-i18n generated render as their key, or with a placeholder
// the params do not give, in every language that could render them.
//
// It checks what an error would render as if it were rendered with the params
// it is made with, by any of the messages packages of the module: whether it is
// rendered at all, or is wrapped or logged instead, it does not follow.
package vet

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/gofile"
	"github.com/go-kanna/internal/packages"
	"github.com/go-kanna/internal/suggest"
	"golang.org/x/mod/modfile"
	gopackages "golang.org/x/tools/go/packages"

	"github.com/go-kanna/i18n"
	"github.com/go-kanna/i18n/internal/gen"
	"github.com/go-kanna/i18n/internal/runtime"
)

const (
	// generator names kanna-i18n in the header of the file it writes.
	generator = "kanna-i18n"

	// generatedFile is the file kanna-i18n writes into a messages package.
	generatedFile = "i18n_gen.go"

	// validationPrefix prefixes the key of the text of a validation error's code.
	validationPrefix = "errors.messages"
)

// reporter is a function reporting a code, by the arguments holding the prefix
// of the key of its text, the code, and the params, -1 for none. Without a
// prefix, it reports a validation error, whose key is errors.messages.<code>.
type reporter struct {
	prefix, code, params int
}

// reporters are the functions vet checks, by their full names.
var reporters = map[string]reporter{
	"github.com/go-kanna/validator.Fail":      {prefix: -1, code: 0, params: -1},
	"github.com/go-kanna/validator.FailWith":  {prefix: -1, code: 0, params: 1},
	"github.com/go-kanna/value/rule.Fail":     {prefix: -1, code: 0, params: -1},
	"github.com/go-kanna/value/rule.FailWith": {prefix: -1, code: 0, params: 1},
	"github.com/go-kanna/i18n.Coded":          {prefix: 0, code: 1, params: 2},
}

// Run checks the packages patterns match in dir against the messages packages
// kanna-i18n generated in the module of dir, and returns what it finds.
func Run(dir, tags string, patterns []string) []diag.Diag {
	set, err := packages.Load(patterns, packages.Config{Dir: dir, Tags: tags, TypesInfo: true})
	if err != nil {
		return []diag.Diag{diag.Errorf(token.Position{}, "%v", err)}
	}
	if diag.HasErrors(set.Diags) {
		return set.Diags
	}
	if len(set.Packages) == 0 {
		return []diag.Diag{diag.Errorf(token.Position{}, "%s matched no packages", strings.Join(patterns, " "))}
	}
	msgs, ds := readMessages(dir, tags)
	if diag.HasErrors(ds) {
		return ds
	}

	c := checker{fset: set.Fset, msgs: msgs}
	for _, pkg := range set.Packages {
		c.check(pkg)
	}
	slices.SortFunc(c.diags, func(a, b diag.Diag) int {
		return cmp.Or(strings.Compare(a.Pos.Filename, b.Pos.Filename),
			cmp.Compare(a.Pos.Line, b.Pos.Line), cmp.Compare(a.Pos.Column, b.Pos.Column))
	})
	return slices.Concat(set.Diags, ds, c.diags)
}

// messages is what the messages packages of a module hold.
type messages struct {
	// packages are the packages, as a diagnostic names them.
	packages string
	// bundles are the Bundles of the packages.
	bundles []i18n.Bundle
	// keys are the keys some language of them renders, which a hint may
	// suggest.
	keys []string
}

// lookup returns the entries every language of every package renders key with,
// one for each that has it.
func (m messages) lookup(key string) []i18n.Entry {
	entries := make([]i18n.Entry, 0, len(m.bundles))
	for _, b := range m.bundles {
		entries = append(entries, runtime.Lookup[i18n.Entry](b, key)...)
	}
	return entries
}

// readMessages reads the packages of the module of dir holding the file
// kanna-i18n generated.
func readMessages(dir, tags string) (messages, []diag.Diag) {
	errorf := func(format string, args ...any) (messages, []diag.Diag) {
		return messages{}, []diag.Diag{diag.Errorf(token.Position{}, format, args...)}
	}
	root, modPath, err := module(dir)
	if err != nil {
		return errorf("%v", err)
	}
	var flags []string
	if tags != "" {
		flags = []string{"-tags=" + tags}
	}
	listed, err := gopackages.Load(&gopackages.Config{
		Dir: root, BuildFlags: flags, Mode: gopackages.NeedName | gopackages.NeedFiles,
	}, "./...")
	if err != nil {
		return errorf("%v", err)
	}
	var paths, names []string
	for _, p := range listed {
		if slices.ContainsFunc(p.GoFiles, generated) {
			paths = append(paths, p.PkgPath)
			name, _ := strings.CutPrefix(p.PkgPath, modPath+"/")
			names = append(names, name)
		}
	}
	if len(paths) == 0 {
		return errorf("no package of module %s holds what %s generates", modPath, generator)
	}

	set, err := packages.Load(paths, packages.Config{Dir: root, Tags: tags, TypesInfo: true})
	if err != nil {
		return errorf("%v", err)
	}
	if diag.HasErrors(set.Diags) {
		return messages{}, set.Diags
	}
	m := messages{packages: strings.Join(names, ", ")}
	keys := map[string]bool{}
	for _, p := range set.Packages {
		i := slices.IndexFunc(p.CompiledGoFiles, generated)
		b, _, ds := gen.ReadBundle(set.Fset, p.Syntax[i], p.TypesInfo)
		if len(ds) > 0 {
			return messages{}, ds
		}
		m.bundles = append(m.bundles, b)
		for _, key := range runtime.Keys(b) {
			keys[key] = true
		}
	}
	m.keys = slices.Sorted(maps.Keys(keys))
	return m, set.Diags
}

// generated reports whether path is the file kanna-i18n generated.
func generated(path string) bool {
	if filepath.Base(path) != generatedFile {
		return false
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly|parser.ParseComments)
	return err == nil && gofile.GeneratedBy(f, generator)
}

// module returns the root and the path of the module holding dir.
func module(dir string) (string, string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("find the module: %w", err)
	}
	for d := abs; ; d = filepath.Dir(d) {
		data, err := os.ReadFile(filepath.Join(d, "go.mod")) //nolint:gosec // the go.mod of the module checked
		if err == nil {
			return d, modfile.ModulePath(data), nil
		}
		if filepath.Dir(d) == d {
			return "", "", fmt.Errorf("%s is in no module", abs)
		}
	}
}

// checker reports the codes the packages it checks report that no language
// renders, or that every language leaves a placeholder in.
type checker struct {
	fset  *token.FileSet
	msgs  messages
	diags []diag.Diag
}

func (c *checker) check(pkg *packages.Package) {
	for _, f := range pkg.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				c.call(pkg.TypesInfo, call)
			}
			return true
		})
	}
}

// call checks call, if it calls a reporter with a constant code.
func (c *checker) call(info *types.Info, call *ast.CallExpr) {
	fn := callee(info, call)
	if fn == nil {
		return
	}
	r, ok := reporters[fn.FullName()]
	if !ok {
		return
	}
	prefix := validationPrefix
	if r.prefix >= 0 {
		if prefix, ok = constString(info, call.Args[r.prefix]); !ok {
			return
		}
	}
	code, ok := constString(info, call.Args[r.code])
	if !ok {
		return
	}
	key := code
	if prefix != "" {
		key = prefix + "." + code
	}

	entries := c.msgs.lookup(key)
	if len(entries) == 0 {
		c.noText(call.Args[r.code], prefix, code, key)
		return
	}
	params, ok := givenParams(info, call, r)
	if !ok {
		return
	}
	at := call.Args[r.code]
	if r.params >= 0 {
		at = call.Args[r.params]
	}
	for _, name := range missing(entries, params) {
		c.diags = append(c.diags, diag.Errorf(c.fset.Position(at.Pos()),
			"%s renders \"{%s}\" in every language of %s that has its text: the call gives no %s",
			key, name, c.msgs.packages, name))
	}
}

// noText reports code, at e, as having no text in any language, suggesting a
// code that has.
func (c *checker) noText(e ast.Expr, prefix, code, key string) {
	d := diag.Errorf(c.fset.Position(e.Pos()), "%q has no text in any language of %s, so it renders as %s",
		code, c.msgs.packages, key)
	codes := c.msgs.keys
	if prefix != "" {
		codes = nil
		for _, k := range c.msgs.keys {
			if code, ok := strings.CutPrefix(k, prefix+"."); ok {
				codes = append(codes, code)
			}
		}
	}
	if s, ok := suggest.Closest(code, codes); ok {
		d = d.WithHints(fmt.Sprintf("did you mean %q?", s))
	}
	c.diags = append(c.diags, d)
}

// callee returns the function call calls by name, or nil.
func callee(info *types.Info, call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	}
	fn, _ := info.Uses[id].(*types.Func)
	return fn
}

// givenParams returns the names of the params call gives, and false when they
// are known only when the program runs: when they are neither nil nor a map
// literal of constant keys.
func givenParams(info *types.Info, call *ast.CallExpr, r reporter) (map[string]bool, bool) {
	names := map[string]bool{}
	if r.params < 0 {
		return names, true
	}
	arg := call.Args[r.params]
	if info.Types[arg].IsNil() {
		return names, true
	}
	lit, ok := ast.Unparen(arg).(*ast.CompositeLit)
	if !ok {
		return nil, false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, false
		}
		name, ok := constString(info, kv.Key)
		if !ok {
			return nil, false
		}
		names[name] = true
	}
	return names, true
}

// missing returns, sorted, the placeholders params does not give that every
// form of entries a call with params may render with uses: every form of a
// plural message when params give the count, which picks one, and Other alone
// when they do not.
func missing(entries []i18n.Entry, params map[string]bool) []string {
	var common map[string]bool
	for _, e := range entries {
		for _, t := range forms(e, params[runtime.CountParam]) {
			used := map[string]bool{}
			for _, s := range t {
				if s.Param != "" && !params[s.Param] {
					used[s.Param] = true
				}
			}
			if common == nil {
				common = used
				continue
			}
			maps.DeleteFunc(common, func(name string, _ bool) bool { return !used[name] })
		}
	}
	return slices.Sorted(maps.Keys(common))
}

// forms returns the forms of e a call may render it with.
func forms(e i18n.Entry, counted bool) []i18n.Template {
	switch {
	case e.Plural == nil:
		return []i18n.Template{e.Single}
	case !counted:
		return []i18n.Template{e.Plural.Other}
	}
	p := e.Plural
	return slices.DeleteFunc([]i18n.Template{p.Zero, p.One, p.Two, p.Few, p.Many, p.Other},
		func(t i18n.Template) bool { return t == nil })
}

// constString returns the constant string e is, and false when it is none.
func constString(info *types.Info, e ast.Expr) (string, bool) {
	v := info.Types[e].Value
	if v == nil || v.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(v), true
}
