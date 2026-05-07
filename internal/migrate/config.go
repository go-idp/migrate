package migrate

import (
	"errors"
	"fmt"
	"strings"
)

type Config struct {
	Driver string
	Host   string
	Port   string
	User   string
	Pass   string
	Name   string
}

// Validate ensures all required database connection fields are present.
func (c Config) Validate() error {
	missing := make([]string, 0, 6)
	if strings.TrimSpace(c.Driver) == "" {
		missing = append(missing, "DB_DRIVER/-D")
	}
	if strings.TrimSpace(c.Host) == "" {
		missing = append(missing, "DB_HOST/-h")
	}
	if strings.TrimSpace(c.Port) == "" {
		missing = append(missing, "DB_PORT/-P")
	}
	if strings.TrimSpace(c.User) == "" {
		missing = append(missing, "DB_USER/-u")
	}
	if strings.TrimSpace(c.Pass) == "" {
		missing = append(missing, "DB_PASS/-p")
	}
	if strings.TrimSpace(c.Name) == "" {
		missing = append(missing, "DB_NAME/-d")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required database configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

// BuildDSN converts the generic config into a driver-specific DSN string.
func BuildDSN(cfg Config) (string, error) {
	driver := normalizeDriver(cfg.Driver)
	switch driver {
	case "mysql", "mariadb":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?multiStatements=true&parseTime=true", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name), nil
	case "postgres":
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Pass, cfg.Name), nil
	case "sqlite3":
		return cfg.Name, nil
	default:
		return "", errors.New("unsupported driver, supported values: mysql, mariadb, postgres, sqlite3")
	}
}

// SQLDriverName maps user-facing driver names to database/sql driver names.
func SQLDriverName(driver string) (string, error) {
	switch normalizeDriver(driver) {
	case "mysql", "mariadb":
		return "mysql", nil
	case "postgres":
		return "postgres", nil
	case "sqlite3":
		return "sqlite", nil
	default:
		return "", errors.New("unsupported driver, supported values: mysql, mariadb, postgres, sqlite3")
	}
}

// normalizeDriver converts driver aliases into canonical names.
func normalizeDriver(driver string) string {
	value := strings.ToLower(strings.TrimSpace(driver))
	switch value {
	case "postgresql":
		return "postgres"
	case "sqlite":
		return "sqlite3"
	default:
		return value
	}
}
