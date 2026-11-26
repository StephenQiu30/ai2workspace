// Package platform wires shared infrastructure for the backend.
package platform

import (
	"database/sql"
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenDatabase configures the connection pool. Readiness is checked on request.
func OpenDatabase(dsn string) (*gorm.DB, *sql.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, nil, errors.New("cannot configure PostgreSQL; check STUDIO_DATABASE_URL")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, pool, nil
}
