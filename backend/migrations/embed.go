// Package migrations embeds schema additions used by the inherited startup migrator.
package migrations

import _ "embed"

// OrderAccess adds hashed, expiring credentials without granting access to old orders.
//
//go:embed 002_order_access.sql
var OrderAccess string
