// Command example renders a shopping cart's messages for readers asking for
// different languages, showing what kanna-i18n generates: a typed constructor
// for every message, and the translations themselves, so no locale file is
// read at run time and there is nothing to set up.
//
// Regenerate with:
//
//	go generate ./...
package main

import (
	"fmt"
	"os"

	"github.com/go-kanna/i18n"

	"github.com/go-kanna/i18n/example/messages"
)

//go:generate go tool kanna-i18n

func main() {
	// A Message names what to say, not how: whatever builds one need not know
	// who will read it.
	cart := []i18n.Message{
		messages.Hello("Alice"),
		messages.ItemsCount(0),       // zero, in every language that writes one
		messages.ItemsCount(1),       // one in English; Japanese says it with other
		messages.ItemsCount(3),       // other
		messages.TotalPrice(1234.56), // with the digits grouped as the language groups them
		messages.UserDeleted("Bob"),  // missing from ja.yaml, so it falls back to English
	}

	// Each is what a browser might send as its Accept-Language. French is not
	// compiled in, so the third reader gets the next language asked for, and the
	// last one, asking for nothing compiled in, gets English, the default.
	for _, accept := range []string{"en-US", "ja", "fr, ja;q=0.8", "de"} {
		loc := messages.Localizer(accept)
		fmt.Fprintf(os.Stdout, "=== Accept-Language: %s ===\n", accept)
		for _, m := range cart {
			fmt.Fprintln(os.Stdout, loc.Localize(m))
		}
		fmt.Fprintln(os.Stdout)
	}
}
