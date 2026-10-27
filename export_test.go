package i18n

// Render renders e in lang with args, as a Localizer does once it has found the
// message in the catalog of lang.
func Render(lang string, e Entry, args ...Arg) (string, error) {
	l, err := newLocale(Catalog{Lang: lang})
	if err != nil {
		return "", err
	}
	return l.render(e, args), nil
}
