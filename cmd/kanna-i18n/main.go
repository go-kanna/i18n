// Command kanna-i18n compiles a directory of locale files into a package of
// typed message constructors, with the translations themselves compiled in.
// kanna-i18n vet finds the error codes that render as their key, or with a
// placeholder the params do not give.
package main

import (
	"os"

	"github.com/go-kanna/i18n/internal/gen"
	"github.com/go-kanna/i18n/internal/vet"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "vet" {
		os.Exit(vet.Command().Main(os.Args[2:]))
	}
	os.Exit(gen.Command().Main(os.Args[1:]))
}
