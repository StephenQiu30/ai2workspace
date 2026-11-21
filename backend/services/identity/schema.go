// Package identity exposes embedded SQL schema assets for the identity service.
package identity

import _ "embed"

// SchemaSQL holds the idempotent PostgreSQL DDL for the identity service.
//
//go:embed schema.sql
var SchemaSQL string
