package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/infrastructure/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "event operation failed; queue data was retained")
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 2 && os.Args[1] == "cleanup-test" {
		cfg, e := config.LoadWorker(os.Getenv, os.Getenv("APP_ENV"))
		if e != nil {
			return e
		}
		if os.Getenv("APP_ENV") != "test" || !regexp.MustCompile(`^ecom_e2e_[0-9a-f]{32}$`).MatchString(cfg.Prefix) {
			return fmt.Errorf("refusing non-test broker cleanup")
		}
		b, e := rabbitmq.Open(cfg.URL, cfg.Prefix)
		if e != nil {
			return e
		}
		defer b.Close()
		ch, e := b.Conn.Channel()
		if e != nil {
			return e
		}
		defer ch.Close()
		for _, purpose := range []string{"notifications", "catalog"} {
			for _, suffix := range []string{"", ".retry", ".dlq"} {
				if _, e = ch.QueueDelete(b.Queue(purpose)+suffix, false, false, false); e != nil {
					return e
				}
			}
		}
		return ch.ExchangeDelete(cfg.Prefix+".events", false, false)
	}

	if len(os.Args) != 3 || (os.Args[1] != "inspect" && os.Args[1] != "replay") || (os.Args[2] != "notifications" && os.Args[2] != "catalog") {
		return fmt.Errorf("usage: events inspect|replay notifications|catalog")
	}
	cfg, e := config.LoadWorker(os.Getenv, os.Getenv("APP_ENV"))
	if e != nil {
		return e
	}
	b, e := rabbitmq.Open(cfg.URL, cfg.Prefix)
	if e != nil {
		return e
	}
	defer b.Close()
	ch, e := b.Conn.Channel()
	if e != nil {
		return e
	}
	defer ch.Close()
	q := b.Queue(os.Args[2])
	d, ok, e := ch.Get(q+".dlq", false)
	if e != nil || !ok {
		return e
	}
	var envelope struct {
		ID      string `json:"event_id"`
		Type    string `json:"type"`
		Version int    `json:"version"`
	}
	_ = json.Unmarshal(d.Body, &envelope)
	if os.Args[1] == "inspect" {
		fmt.Printf("event_id=%s type=%s version=%d\n", envelope.ID, envelope.Type, envelope.Version)
		return d.Nack(false, true)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if e = b.Publish(ctx, "", q, amqp.Publishing{ContentType: d.ContentType, MessageId: d.MessageId, Body: d.Body, Headers: amqp.Table{"attempt": int32(0)}}); e != nil {
		return e
	}
	if e = d.Ack(false); e != nil {
		return e
	}
	fmt.Println("Replayed one message with its original event ID")
	return nil
}
