//go:build !dev

package api

import "embed"

//go:embed all:dist
var distFS embed.FS

var useEmbed = true
