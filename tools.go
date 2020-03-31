//go:generate go run code.soquee.net/pkgzip -m -f -src assets -pkg assetfs

// +build tools

package main

import (
	_ "code.soquee.net/pkgzip"
)
