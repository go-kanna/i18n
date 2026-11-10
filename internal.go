package i18n

import "github.com/go-kanna/i18n/internal/runtime"

func init() { //nolint:gochecknoinits // internal/runtime cannot import this package to read them
	runtime.IsBuiltin = isBuiltin
	runtime.BuiltinIn = builtinIn
	runtime.BuiltinParams = builtinParams
	runtime.SetBundle(lookupEntries, bundleKeys)
}
