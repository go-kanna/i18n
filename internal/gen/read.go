package gen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/go-kanna/internal/diag"

	"github.com/go-kanna/i18n"
)

// ReadBundle builds the Bundle f, a file kanna-i18n generated, builds from the
// catalogs it passes i18n.NewBundle, and returns it with the catalogs; info
// holds the types of f's package. It reads the literals emit writes by the
// constants they hold, however they are laid out. Any other form, and catalogs
// NewBundle rejects, is reported where it is, rather than read as fewer
// catalogs than the program renders with.
func ReadBundle(fset *token.FileSet, f *ast.File, info *types.Info) (i18n.Bundle, []i18n.Catalog, []diag.Diag) {
	catalogs, call, err := reader{info: info}.catalogs(f)
	var b i18n.Bundle
	if err == nil {
		b, err = newBundle(call, catalogs)
	}
	var re readError
	if !errors.As(err, &re) {
		return b, catalogs, nil
	}
	d := diag.Errorf(fset.Position(re.pos), "cannot read the catalogs %s generated: %s", generator, re.msg)
	return i18n.Bundle{}, nil, []diag.Diag{d.WithHints("regenerate the file with " + generator)}
}

// readError is a form ReadBundle cannot read, at pos.
type readError struct {
	pos token.Pos
	msg string
}

func (e readError) Error() string {
	return e.msg
}

func errorAt(n ast.Node, format string, args ...any) error {
	return readError{pos: n.Pos(), msg: fmt.Sprintf(format, args...)}
}

func notLiteral(e ast.Expr) error {
	return errorAt(e, "%s is not a literal", types.ExprString(e))
}

// newBundle calls i18n.NewBundle as call does, reporting at call the catalogs
// it panics on, as the program would at its start: one of an invalid language,
// or two of the same.
func newBundle(call *ast.CallExpr, catalogs []i18n.Catalog) (b i18n.Bundle, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = errorAt(call, "i18n.NewBundle panics with %v", p)
		}
	}()
	return i18n.NewBundle(catalogs[0], catalogs[1:]...), nil
}

type reader struct {
	info *types.Info
}

// catalogs reads the catalogs f passes i18n.NewBundle, and the call.
func (r reader) catalogs(f *ast.File) ([]i18n.Catalog, *ast.CallExpr, error) {
	var calls []*ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && r.callsNewBundle(c) {
			calls = append(calls, c)
		}
		return true
	})
	switch {
	case len(calls) == 0:
		return nil, nil, errorAt(f.Name, "the file does not call i18n.NewBundle")
	case len(calls) > 1:
		return nil, nil, errorAt(calls[1], "the file calls i18n.NewBundle a second time")
	case calls[0].Ellipsis.IsValid():
		return nil, nil, errorAt(calls[0], "the catalogs are passed as a slice")
	}

	call := calls[0]
	catalogs := make([]i18n.Catalog, len(call.Args))
	for i, arg := range call.Args {
		c, err := r.catalog(arg)
		if err != nil {
			return nil, nil, err
		}
		catalogs[i] = c
	}
	return catalogs, call, nil
}

// callsNewBundle reports whether c calls i18n.NewBundle, told by its type
// rather than by names, which the imports and declarations of the package can
// change.
func (r reader) callsNewBundle(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := r.info.Uses[sel.Sel].(*types.Func)
	return ok && fn.FullName() == runtimePath+".NewBundle"
}

func (r reader) catalog(e ast.Expr) (i18n.Catalog, error) {
	fields, err := r.fields(e)
	if err != nil {
		return i18n.Catalog{}, err
	}
	var c i18n.Catalog
	for _, f := range fields {
		switch f.name {
		case "Lang":
			c.Lang, err = r.str(f.value)
		case "Entries":
			c.Entries, err = r.entries(f.value)
		}
		if err != nil {
			return i18n.Catalog{}, err
		}
	}
	return c, nil
}

func (r reader) entries(e ast.Expr) (map[string]i18n.Entry, error) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, notLiteral(e)
	}
	entries := make(map[string]i18n.Entry, len(lit.Elts))
	for _, elt := range lit.Elts {
		kv := elt.(*ast.KeyValueExpr) //nolint:forcetypeassert // every element of a map literal has its key
		key, err := r.str(kv.Key)
		if err != nil {
			return nil, err
		}
		if entries[key], err = r.entry(kv.Value); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func (r reader) entry(e ast.Expr) (i18n.Entry, error) {
	fields, err := r.fields(e)
	if err != nil {
		return i18n.Entry{}, err
	}
	var en i18n.Entry
	for _, f := range fields {
		switch f.name {
		case "Single":
			en.Single, err = r.template(f.value)
		case "Plural":
			en.Plural, err = r.plural(f.value)
		}
		if err != nil {
			return i18n.Entry{}, err
		}
	}
	return en, nil
}

func (r reader) plural(e ast.Expr) (*i18n.Plural, error) {
	addr, ok := e.(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		return nil, notLiteral(e)
	}
	fields, err := r.fields(addr.X)
	if err != nil {
		return nil, err
	}
	var p i18n.Plural
	forms := map[string]*i18n.Template{
		"Zero": &p.Zero, "One": &p.One, "Two": &p.Two, "Few": &p.Few, "Many": &p.Many, "Other": &p.Other,
	}
	for _, f := range fields {
		if *forms[f.name], err = r.template(f.value); err != nil {
			return nil, err
		}
	}
	return &p, nil
}

func (r reader) template(e ast.Expr) (i18n.Template, error) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, notLiteral(e)
	}
	t := make(i18n.Template, len(lit.Elts))
	for i, elt := range lit.Elts {
		fields, err := r.fields(elt)
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			switch f.name {
			case "Text":
				t[i].Text, err = r.str(f.value)
			case "Param":
				t[i].Param, err = r.str(f.value)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return t, nil
}

// field is a field a struct literal gives, and its value.
type field struct {
	name  string
	value ast.Expr
}

// fields reads e as a struct literal, keyed or not.
func (r reader) fields(e ast.Expr) ([]field, error) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return nil, notLiteral(e)
	}
	fields := make([]field, len(lit.Elts))
	for i, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			fields[i] = field{name: kv.Key.(*ast.Ident).Name, value: kv.Value} //nolint:forcetypeassert // a field name
			continue
		}
		st := r.info.TypeOf(lit).Underlying().(*types.Struct) //nolint:forcetypeassert // the literal is of a struct
		fields[i] = field{name: st.Field(i).Name(), value: elt}
	}
	return fields, nil
}

// str reads e as the constant string it is.
func (r reader) str(e ast.Expr) (string, error) {
	v := r.info.Types[e].Value
	if v == nil || v.Kind() != constant.String {
		return "", errorAt(e, "%s is not a constant string", types.ExprString(e))
	}
	return constant.StringVal(v), nil
}
