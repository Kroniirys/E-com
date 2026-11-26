package config

import (
	"testing"
)

func TestWorkerConfiguration(t *testing.T) {
	env := map[string]string{"RABBITMQ_URL": "amqp://localhost:5672/", "REDIS_URL": "redis://localhost:6379/"}
	get := func(k string) string { return env[k] }
	if _, e := LoadWorker(get, "test"); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadWorker(get, "production"); e == nil {
		t.Fatal("production accepted plaintext AMQP")
	}
	env["RABBITMQ_URL"] = "amqps://localhost:5671/"
	if _, e := LoadWorker(get, "production"); e == nil {
		t.Fatal("production worker accepted plaintext Redis")
	}
	env["REDIS_URL"] = "rediss://localhost:6379/"
	if _, e := LoadWorker(get, "production"); e != nil {
		t.Fatal(e)
	}
	env["BROKER_PREFIX"] = "invalid.namespace"
	if _, e := LoadWorker(get, "test"); e == nil {
		t.Fatal("invalid namespace accepted")
	}
}
