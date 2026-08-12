package model

import "strings"

// ExtensionSet is the set of PostgreSQL extensions installed in the target
// database, by name. A read that fails degrades to an empty set — treated
// exactly like "nothing installed" rather than an error — so a recommendation
// gated on an extension never turns absence into a crash.
type ExtensionSet map[string]bool

// NewExtensionSet builds a set from the extension names read from the
// catalog.
func NewExtensionSet(names []string) ExtensionSet {
	out := make(ExtensionSet, len(names))
	for _, n := range names {
		out[strings.ToLower(n)] = true
	}
	return out
}

// Has reports whether the named extension is installed.
func (e ExtensionSet) Has(name string) bool {
	return e[strings.ToLower(name)]
}
