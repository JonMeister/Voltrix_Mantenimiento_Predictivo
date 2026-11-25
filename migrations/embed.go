package migrations

import "embed"

// Files contiene las migraciones SQL y datos de ejemplo embebidos.
//
//go:embed *.sql
var Files embed.FS

