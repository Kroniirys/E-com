package config

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddr        string
	Version         string
	DatabaseURL     string
	AllowedOrigins  []string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	DBTimeout       time.Duration
}

func Load(get func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		if v := get(key); v != "" {
			return v
		}
		return fallback
	}
	c := Config{Environment: value("APP_ENV", "development"), HTTPAddr: value("HTTP_ADDR", ":8080"), Version: value("APP_VERSION", "phase05"), DatabaseURL: get("DATABASE_URL")}
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return c, errors.New("APP_ENV must be development, test or production")
	}
	if _, port, err := net.SplitHostPort(c.HTTPAddr); err != nil {
		return c, errors.New("HTTP_ADDR must include host and port")
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return c, errors.New("HTTP_ADDR port is invalid")
	}
	if err := ValidateDatabaseURL(c.DatabaseURL); err != nil {
		return c, err
	}
	u, _ := url.Parse(c.DatabaseURL)
	if c.Environment == "production" && u.Query().Get("sslmode") != "verify-full" {
		return c, errors.New("production DATABASE_URL requires sslmode=verify-full")
	}
	for _, entry := range strings.Split(get("ALLOWED_ORIGINS"), ",") {
		origin := strings.TrimSpace(entry)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return c, errors.New("ALLOWED_ORIGINS must contain exact HTTP origins without paths")
		}
		if c.Environment == "production" && parsed.Scheme != "https" {
			return c, errors.New("production origins require HTTPS")
		}
		c.AllowedOrigins = append(c.AllowedOrigins, origin)
	}
	durations := []struct {
		key, fallback string
		target        *time.Duration
	}{{"REQUEST_TIMEOUT", "5s", &c.RequestTimeout}, {"SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout}, {"DB_TIMEOUT", "3s", &c.DBTimeout}}
	for _, d := range durations {
		parsed, err := time.ParseDuration(value(d.key, d.fallback))
		if err != nil || parsed <= 0 || parsed > time.Minute {
			return c, errors.New(d.key + " must be positive and at most one minute")
		}
		*d.target = parsed
	}
	return c, nil
}

func ValidateDatabaseURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.User == nil || u.User.Username() == "" || !regexp.MustCompile(`^/[A-Za-z0-9_]+$`).MatchString(u.Path) || u.Fragment != "" {
		return errors.New("DATABASE_URL must be a valid PostgreSQL URL with user, host and database")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("DATABASE_URL query is invalid")
	}
	allowed := map[string]bool{"sslmode": true, "search_path": true, "connect_timeout": true, "application_name": true, "sslrootcert": true, "sslcert": true, "sslkey": true}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			return errors.New("DATABASE_URL contains unsupported or repeated options")
		}
	}
	return nil
}
