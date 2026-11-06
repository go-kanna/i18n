package vet

import "github.com/go-kanna/internal/cli"

// Command returns kanna-i18n vet.
func Command() cli.Vet {
	return cli.Vet{
		Name:    generator,
		Summary: "find the error codes that render as their key, or with a placeholder the params do not give",
		Run:     Run,
	}
}
