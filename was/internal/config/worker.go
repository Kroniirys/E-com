package config

import (
	"errors"
	"net/url"
	"regexp"
)

type Worker struct{ URL, Prefix, Address string }

func LoadWorker(get func(string) string, environment string) (Worker, error) {
	c := Worker{get("RABBITMQ_URL"), get("BROKER_PREFIX"), get("WORKER_HTTP_ADDR")}
	if c.Prefix == "" {
		c.Prefix = "ecommerce"
	}
	if c.Address == "" {
		c.Address = ":8082"
	}
	u, e := url.Parse(c.URL)
	if e != nil || u.Hostname() == "" || (u.Scheme != "amqp" && u.Scheme != "amqps") || (environment == "production" && u.Scheme != "amqps") {
		return c, errors.New("RABBITMQ_URL must be AMQP (TLS in production)")
	}
	if raw := get("REDIS_URL"); raw != "" {
		redisURL, e := url.Parse(raw)
		if e != nil || redisURL.Hostname() == "" || (redisURL.Scheme != "redis" && redisURL.Scheme != "rediss") || (environment == "production" && redisURL.Scheme != "rediss") {
			return c, errors.New("worker Redis URL requires TLS in production")
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(c.Prefix) {
		return c, errors.New("invalid BROKER_PREFIX")
	}
	return c, nil
}
