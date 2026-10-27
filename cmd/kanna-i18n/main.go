// Command kanna-i18n compiles a directory of locale files into a package of
// typed message constructors, with the translations themselves compiled in.
package main

import (
	"os"

	"github.com/go-kanna/i18n/internal/gen"
)

func main() {
	os.Exit(gen.Command().Main(os.Args[1:]))
}
