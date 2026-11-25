package migrations

import (
	"embed"
	"errors"
	"net/url"

	"ecommerce.local/base/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

func New(databaseURL string) (*migrate.Migrate, error) {
	source, err := iofs.New(files, ".")
	if err != nil {
		return nil, errors.New("migration source unavailable")
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		_ = source.Close()
		return nil, errors.New("migration connection failed")
	}
	return m, nil
}

func Up(m *migrate.Migrate) error {
	err := m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func AllowTestReset(environment, databaseURL, confirmation string) bool {
	u, err := url.Parse(databaseURL)
	return config.ValidateDatabaseURL(databaseURL) == nil && err == nil && u != nil && environment == "test" && confirmation == "delete-test-schema" && len(u.Path) > 5 && u.Path[len(u.Path)-5:] == "_test"
}
