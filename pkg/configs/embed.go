// Package configs embeds static data shipped with Dotty, notably the default
// package catalog. Keeping it in a separate package avoids an import cycle:
// catalog depends on configs, but configs depends on nothing in internal/.
package configs

import _ "embed"

// PackagesJSON is the built-in default package catalog. Users may override it
// at runtime by placing a packages.json in Dotty's config directory.
//
//go:embed packages.json
var PackagesJSON []byte
