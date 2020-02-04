// +build tools

package tools

import (
	_ "golang.org/x/lint/golint"
	_ "golang.org/x/tools/cmd/cover"
	_ "github.com/mattn/goveralls"
)
