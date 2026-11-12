// Package runtime is what kanna-i18n reads of package i18n but a program does
// not: package i18n sets it when it is initialized, so that the generator and
// vet see the built-in texts and the languages a Bundle searches as it does.
package runtime

import "golang.org/x/text/language"

// CountParam is the argument whose value, an integer, picks the form of a plural
// message. kanna-i18n names the count of every plural constructor so.
const CountParam = "count"

var (
	// IsBuiltin reports whether key is a built-in text, which the catalog of
	// any language may override, the default language's included or not.
	IsBuiltin func(key string) bool
	// BuiltinIn reports whether the built-in texts of the language of lang,
	// those of en for en-GB, hold key.
	BuiltinIn func(lang language.Tag, key string) bool
	// BuiltinParams returns, sorted, the names of the placeholders the
	// built-in text key fills in any language, and false when key is no
	// built-in text.
	BuiltinParams func(key string) ([]string, bool)
)

// lookup and keys are func(i18n.Bundle, string) []i18n.Entry and
// func(i18n.Bundle) []string, which this package, imported by package i18n,
// cannot name.
var lookup, keys any

// SetBundle is how package i18n sets what Lookup and Keys call.
func SetBundle[B, E any](l func(B, string) []E, k func(B) []string) {
	lookup, keys = l, k
}

// Lookup returns the entries the languages of b render key with, one for each
// language that has key, the built-in texts included; a language with none
// renders key as itself. The entries are copies, so changing one changes
// nothing b renders.
func Lookup[E, B any](b B, key string) []E {
	return lookup.(func(B, string) []E)(b, key) //nolint:forcetypeassert // package i18n sets it with its Bundle and Entry
}

// Keys returns, sorted, the keys some language of b renders, the built-in texts
// included where a language reaches them: those Lookup finds an entry of.
func Keys[B any](b B) []string {
	return keys.(func(B) []string)(b) //nolint:forcetypeassert // package i18n sets it with its Bundle
}
