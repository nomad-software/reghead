// Package appicon embeds the program's icon in the binary.
package appicon

import _ "embed"

// PNG is the program's icon, a 256x256 PNG.
//
//go:embed appicon.png
var PNG []byte
