package gen

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/suggest"
	"go.yaml.in/yaml/v3"

	"github.com/go-kanna/i18n"
)

// pluralKey is the key of the mapping that holds the forms of a plural
// message.
const pluralKey = "plural"

// categories are the CLDR plural categories, in their canonical order, as the
// mapping under pluralKey spells them.
var categories = []string{"zero", "one", "two", "few", "many", "other"}

// entry is one message of a locale file.
type entry struct {
	key string
	// pos is where the key is written.
	pos token.Position
	// forms holds the message: one form without a category or, for a plural
	// message, one form per category written, in the order of categories.
	forms []form
}

// plural reports whether e is a plural message.
func (e entry) plural() bool {
	return e.forms[0].category != ""
}

// form is one form of a message, as written and parsed.
type form struct {
	template

	// category is the plural category of the form, or "" for a message that is
	// not plural.
	category string
	// src is the text as written, which the constructor's doc quotes.
	src string
	// pos is where the text is written.
	pos token.Position
}

// parseLocale reads the messages of data, the locale file at path, in the
// order they are written.
//
// Keys nest: a mapping under a key prefixes the keys in it with that key and a
// dot. The mapping under the key "plural" holds the forms of a plural message
// instead, by category, and nothing else may sit beside it. Every mistake is
// reported where it is written, and a message with one is left out, so that the
// mistake stops generation without hiding the rest.
func parseLocale(path string, data []byte) ([]entry, []diag.Diag) {
	p := localeParser{path: path}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, []diag.Diag{p.syntaxError(err)}
	}
	var next yaml.Node
	switch err := dec.Decode(&next); {
	case err == nil:
		return nil, []diag.Diag{diag.Errorf(p.pos(&next), "a locale file holds one YAML document, and this is a second")}
	case !errors.Is(err, io.EOF):
		return nil, []diag.Diag{p.syntaxError(err)}
	}

	switch root := doc.Content[0]; {
	case root.Tag == "!!null":
		return nil, nil
	case root.Kind != yaml.MappingNode:
		return nil, []diag.Diag{diag.Errorf(p.pos(root), "a locale file maps keys to messages")}
	default:
		p.mapping(root, "")
		return p.entries, p.diags
	}
}

type localeParser struct {
	path    string
	entries []entry
	diags   []diag.Diag
	// open are the mappings being read, the outermost first. An alias to one of
	// them would be read without end: the YAML package rejects such a cycle when
	// decoding into Go values, but not into nodes.
	open []*yaml.Node
}

func (p *localeParser) pos(n *yaml.Node) token.Position {
	return token.Position{Filename: p.path, Line: n.Line, Column: n.Column}
}

func (p *localeParser) report(d diag.Diag) {
	p.diags = append(p.diags, d)
}

// mapping reads the keys of n, each after prefix.
func (p *localeParser) mapping(n *yaml.Node, prefix string) {
	p.open = append(p.open, n)
	seen := make(map[string]bool, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], resolve(n.Content[i+1])
		if k.Kind != yaml.ScalarNode || !validName(k.Value) {
			p.report(diag.Errorf(p.pos(k), "invalid key %q: each part of a key matches [a-z][a-z0-9_]*", k.Value))
			continue
		}
		key := k.Value
		if prefix != "" {
			key = prefix + "." + key
		}
		if seen[k.Value] {
			p.report(diag.Errorf(p.pos(k), "key %q is written twice", key))
			continue
		}
		seen[k.Value] = true

		if v.Kind != yaml.MappingNode {
			if f, ok := p.form(v, key, ""); ok {
				p.entries = append(p.entries, entry{key: key, pos: p.pos(k), forms: []form{f}})
			}
			continue
		}
		if slices.Contains(p.open, v) {
			p.report(diag.Errorf(p.pos(k), "key %q refers to itself through an alias", key))
			continue
		}
		switch forms := pluralForms(v); {
		case forms != nil && len(v.Content) > 2:
			p.report(diag.Errorf(p.pos(k), "key %q holds plural forms and other keys beside them", key).
				WithHints(fmt.Sprintf("%q is the key reserved for the forms of a plural message", pluralKey)))
		case forms != nil:
			p.plural(k, forms, key)
		case len(v.Content) == 0:
			p.report(diag.Errorf(p.pos(v), "key %q holds an empty mapping", key))
		default:
			p.mapping(v, key)
		}
	}
	p.open = p.open[:len(p.open)-1]
}

// plural reads n, the forms of the plural message key, written at k.
func (p *localeParser) plural(k, n *yaml.Node, key string) {
	var (
		forms   []form
		written []string
		failed  bool
	)
	for i := 0; i < len(n.Content); i += 2 {
		c, v := n.Content[i], resolve(n.Content[i+1])
		switch {
		case !slices.Contains(categories, c.Value):
			d := diag.Errorf(p.pos(c), "%q is not a plural category", c.Value)
			if s, ok := suggest.Closest(c.Value, categories); ok {
				d = d.WithHints(fmt.Sprintf("did you mean %q?", s))
			} else {
				d = d.WithHints("a category is zero, one, two, few, many, or other")
			}
			p.report(d)
			failed = true
			continue
		case slices.Contains(written, c.Value):
			p.report(diag.Errorf(p.pos(c), "plural form %q of key %q is written twice", c.Value, key))
			failed = true
			continue
		}
		written = append(written, c.Value)

		f, ok := p.form(v, key, c.Value)
		if !ok {
			failed = true
			continue
		}
		forms = append(forms, f)
	}
	if !slices.Contains(written, "other") {
		p.report(diag.Errorf(p.pos(n), "plural forms of key %q lack %q", key, "other").
			WithHints("other is the form a count without a form of its own renders with"))
		return
	}
	if failed {
		return
	}
	slices.SortFunc(forms, func(a, b form) int {
		return slices.Index(categories, a.category) - slices.Index(categories, b.category)
	})
	p.entries = append(p.entries, entry{key: key, pos: p.pos(k), forms: forms})
}

// form reads v, the text of key: of its plural form category, or, when
// category is "", of the whole message.
func (p *localeParser) form(v *yaml.Node, key, category string) (form, bool) {
	what := fmt.Sprintf("message of key %q", key)
	if category != "" {
		what = fmt.Sprintf("plural form %q of key %q", category, key)
	}
	switch {
	case v.Kind != yaml.ScalarNode:
		p.report(diag.Errorf(p.pos(v), "%s is not a string", what))
		return form{}, false
	case v.Tag == "!!null":
		p.report(diag.Errorf(p.pos(v), "%s is missing", what).WithHints(`write "" for an empty message`))
		return form{}, false
	case v.Tag != "!!str":
		p.report(diag.Errorf(p.pos(v), "%s is not a string", what).
			WithHints(fmt.Sprintf("quote it to have it read as text: %q", v.Value)))
		return form{}, false
	}

	t, ds := parseTemplate(v.Value, p.pos(v))
	if category != "" {
		i := slices.IndexFunc(t.params, func(q param) bool { return q.name == i18n.CountParam })
		if i >= 0 && t.params[i].kind != kindInt && t.params[i].explicit {
			ds = append(ds, diag.Errorf(p.pos(v),
				"%s writes the count as %s, but the count of a plural message is an int", what, t.params[i].kind))
		}
	}
	if len(ds) > 0 {
		p.diags = append(p.diags, ds...)
		return form{}, false
	}
	return form{template: t, category: category, src: v.Value, pos: p.pos(v)}, true
}

// yamlLine matches a YAML syntax error that names its line, which the YAML
// package writes into the message alone.
var yamlLine = regexp.MustCompile(`(?s)^yaml: line (\d+): (.*)$`)

func (p *localeParser) syntaxError(err error) diag.Diag {
	pos := token.Position{Filename: p.path}
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	if m := yamlLine.FindStringSubmatch(err.Error()); m != nil {
		pos.Line, _ = strconv.Atoi(m[1])
		msg = m[2]
	}
	return diag.Errorf(pos, "%s", msg)
}

// pluralForms returns the mapping under pluralKey in n, or nil when there is
// none. A string under pluralKey is an ordinary message, and category names
// anywhere else are ordinary keys.
func pluralForms(n *yaml.Node) *yaml.Node {
	for i := 0; i < len(n.Content); i += 2 {
		if v := resolve(n.Content[i+1]); n.Content[i].Value == pluralKey && v.Kind == yaml.MappingNode {
			return v
		}
	}
	return nil
}

// resolve returns the node n aliases, or n when it is no alias.
func resolve(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}
