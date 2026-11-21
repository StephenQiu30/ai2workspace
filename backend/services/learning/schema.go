// Package learning exposes embedded database schema for the learning service.
package learning

import _ "embed"

// SchemaSQL holds the PostgreSQL schema DDL for the learning service.
//
//go:embed schema.sql
var SchemaSQL string
