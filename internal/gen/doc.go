// Package gen compiles a directory of locale files into a Go package of typed
// message constructors, along with the translations themselves.
// cmd/kanna-i18n runs it.
//
// The default language defines the constructors, and every other language is
// checked against it, so a translation that does not fit stops generation
// instead of the first request that meets it. What the generated code builds is
// rendered by the runtime at the root of this module.
package gen
