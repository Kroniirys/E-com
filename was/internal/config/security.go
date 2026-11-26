package config

import (
	"errors"
	"net/url"
	"regexp"
)

type Security struct {
	Secret       string
	RedisURL     string
	RatePrefix   string
	SecureCookie bool
}

func LoadSecurity(get func(string) string, environment string) (Security, error) {
	c := Security{Secret: get("AUTH_JWT_SECRET"), RedisURL: get("REDIS_URL"), RatePrefix: get("RATE_LIMIT_PREFIX"), SecureCookie: environment == "production"}
	if len(c.Secret) < 32 {
		return c, errors.New("AUTH_JWT_SECRET must contain at least 32 bytes")
	}
	u, err := url.Parse(c.RedisURL)
	if err != nil || u == nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Hostname() == "" {
		return c, errors.New("REDIS_URL must be a valid Redis URL")
	}
	if environment == "production" && u.Scheme != "rediss" {
		return c, errors.New("production REDIS_URL requires TLS")
	}
	if c.RatePrefix == "" {
		c.RatePrefix = "ecommerce:rate:"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9:_-]{1,64}$`).MatchString(c.RatePrefix) {
		return c, errors.New("RATE_LIMIT_PREFIX contains invalid characters")
	}
	return c, nil
}
