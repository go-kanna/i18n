package gen

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/suggest"
)

// template is a message as a locale file writes it, such as "Hello, {name}!",
// split into its text and placeholders.
type template struct {
	segments []segment
	// params are the placeholders, each once, in the order they first appear.
	params []param
}

// segment is a run of text or, when param is set, a placeholder.
type segment struct {
	text  string
	param string
}

// param is a placeholder, which becomes a parameter of the constructor.
type param struct {
	name string
	kind kind
	// explicit reports whether the kind was written out, as in {price:number},
	// rather than left to default. Only a kind written out can conflict with
	// another.
	explicit bool
}

// kind is what a placeholder takes, and so how its argument is rendered.
type kind int

const (
	kindString kind = iota // {name}, {name:string}
	kindInt                // {name:int}
	kindNumber             // {name:number}
)

// kinds spells each kind as a placeholder writes it.
var kinds = []string{kindString: "string", kindInt: "int", kindNumber: "number"}

func (k kind) String() string { return kinds[k] }

// parseTemplate parses src, a message that a locale file writes at pos. It
// reports every mistake it can tell apart, each at pos: the locale file says
// where the message is, not where in it the mistake is.
//
// Braces are doubled for literal ones, "{{" and "}}". A placeholder written
// without a kind takes one written for the same name elsewhere in src.
func parseTemplate(src string, pos token.Position) (template, []diag.Diag) {
	var (
		t     template
		text  strings.Builder
		diags []diag.Diag
	)
	for s := src; s != ""; {
		i := strings.IndexAny(s, "{}")
		if i < 0 {
			text.WriteString(s)
			break
		}
		text.WriteString(s[:i])
		s = s[i:]

		switch {
		case strings.HasPrefix(s, "{{"), strings.HasPrefix(s, "}}"):
			text.WriteByte(s[0])
			s = s[2:]
		case s[0] == '}':
			diags = append(diags, diag.Errorf(pos, `"}" closes no placeholder`).
				WithHints(`write "}}" for a literal "}"`))
			s = s[1:]
		default:
			body, rest, ok := strings.Cut(s[1:], "}")
			if !ok {
				diags = append(diags, diag.Errorf(pos, `"{" opens a placeholder that is not closed`).
					WithHints(`write "{{" for a literal "{"`))
				s = ""
				continue
			}
			s = rest

			p, d, ok := parsePlaceholder(body, pos)
			if !ok {
				diags = append(diags, d)
				continue
			}
			var prev kind
			if t.params, prev, ok = addParam(t.params, p); !ok {
				diags = append(diags, diag.Errorf(pos, "placeholder %s is written as both %s and %s", p.name, prev, p.kind))
				continue
			}
			if text.Len() > 0 {
				t.segments = append(t.segments, segment{text: text.String()})
				text.Reset()
			}
			t.segments = append(t.segments, segment{param: p.name})
		}
	}
	if text.Len() > 0 {
		t.segments = append(t.segments, segment{text: text.String()})
	}
	return t, diags
}

// parsePlaceholder parses body, what a placeholder holds between its braces,
// such as "price:number".
func parsePlaceholder(body string, pos token.Position) (param, diag.Diag, bool) {
	written := "{" + body + "}"
	name, k, hasKind := strings.Cut(body, ":")
	if !validName(name) {
		return param{}, diag.Errorf(pos, "invalid placeholder %s: a name matches [a-z][a-z0-9_]*", written), false
	}
	if !hasKind {
		return param{name: name}, diag.Diag{}, true
	}
	i := slices.Index(kinds, k)
	if i < 0 {
		d := diag.Errorf(pos, "unknown kind %q in %s", k, written)
		if s, ok := suggest.Closest(k, kinds); ok {
			return param{}, d.WithHints(fmt.Sprintf("did you mean %q?", s)), false
		}
		return param{}, d.WithHints("a kind is string, int, or number"), false
	}
	return param{name: name, kind: kind(i), explicit: true}, diag.Diag{}, true
}

// addParam adds p to params, unless a placeholder of its name is there already.
// Then a kind written out for either settles the kind of both, and when both
// write one out and they differ, addParam returns false along with the kind
// written first.
func addParam(params []param, p param) ([]param, kind, bool) {
	i := slices.IndexFunc(params, func(q param) bool { return q.name == p.name })
	switch {
	case i < 0:
		return append(params, p), 0, true
	case !p.explicit:
		return params, 0, true
	case params[i].explicit && params[i].kind != p.kind:
		return params, params[i].kind, false
	default:
		params[i].kind, params[i].explicit = p.kind, true
		return params, 0, true
	}
}

// validName reports whether s, a placeholder or one part of a key, matches
// [a-z][a-z0-9_]*, which turns into a Go name with nothing lost.
func validName(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_'
	})
}
