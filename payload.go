// Package harness embeds the files the CLI installs into a project.
//
// template/ is the base installation, copied as-is. template-clients/<client>/
// holds the fixed files each client adds; the installer generates the rest.
package harness

import "embed"

//go:embed all:template all:template-clients
var Payload embed.FS
