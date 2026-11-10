package i18n

import (
	"maps"
	"slices"
)

// Message is a message to render, with the arguments its placeholders take.
// The constructors kanna-i18n generates return one, so that code that does not
// know the reader's language can hand it to code that does.
type Message struct {
	Key  string
	Args []Arg
}

// Arg is the value of the placeholder Name.
//
// A string is rendered as it is. A number of any integer or float type, named
// or not, is rendered in the number format of the language rendering it: an
// integer in the language's digits without grouping, and a float under CLDR's
// default pattern, which keeps three fraction digits at most. Any other value,
// and a number whose type has a String or Error method, is rendered by fmt.
type Arg struct {
	Name  string
	Value any
}

// Coded returns the message keyed prefix.code, such as
// "errors.messages.too_long", with params as its arguments. It is how an error
// reported as a code and params, as kanna-validator and kanna-value report one,
// gets its text from the locale files; FullMessages and ErrorMessages use it
// for every problem they render. An empty prefix leaves the key code alone.
//
// The arguments come sorted by name, so that one map always makes the same
// Message. Each renders by its type, as Arg says: the kind a locale file gives
// a placeholder types only the generated constructor, which Coded bypasses.
func Coded(prefix, code string, params map[string]any) Message {
	m := Message{Key: code}
	if prefix != "" {
		m.Key = prefix + "." + code
	}
	for _, name := range slices.Sorted(maps.Keys(params)) {
		m.Args = append(m.Args, Arg{Name: name, Value: params[name]})
	}
	return m
}
