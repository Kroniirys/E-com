package rabbitmq

import (
	"context"
	"errors"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Broker struct {
	Conn    *amqp.Connection
	socket  net.Conn
	pub     *amqp.Channel
	returns chan amqp.Return
	gate    chan struct{}
	Prefix  string
}

func Open(raw, prefix string) (*Broker, error) {
	var socket net.Conn
	conn, e := amqp.DialConfig(raw, amqp.Config{Heartbeat: 5 * time.Second, Dial: func(network, address string) (net.Conn, error) {
		c, e := net.DialTimeout(network, address, 3*time.Second)
		socket = c
		if e == nil {
			_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		}
		return c, e
	}})
	if e != nil {
		return nil, errors.New("broker unavailable")
	}
	b := &Broker{Conn: conn, Prefix: prefix, socket: socket, gate: make(chan struct{}, 1)}
	ch, e := conn.Channel()
	if e != nil {
		conn.Close()
		return nil, e
	}
	b.pub = ch
	if e = ch.ExchangeDeclare(prefix+".events", "topic", true, false, false, false, nil); e != nil {
		b.Close()
		return nil, e
	}
	for _, purpose := range []string{"notifications", "catalog"} {
		q := b.Queue(purpose)
		for _, name := range []string{q, q + ".retry", q + ".dlq"} {
			if _, e = ch.QueueDeclare(name, true, false, false, false, nil); e != nil {
				b.Close()
				return nil, e
			}
		}
		patterns := []string{"order.#", "payment.completed"}
		if purpose == "catalog" {
			patterns = []string{"product.changed", "category.changed", "inventory.updated"}
		}
		for _, pattern := range patterns {
			if e = ch.QueueBind(q, pattern, prefix+".events", false, nil); e != nil {
				b.Close()
				return nil, e
			}
		}
	}
	if e = ch.Confirm(false); e != nil {
		b.Close()
		return nil, e
	}
	b.returns = ch.NotifyReturn(make(chan amqp.Return, 1))
	return b, nil
}
func (b *Broker) Queue(purpose string) string { return b.Prefix + "." + purpose + ".v1" }
func (b *Broker) Close()                      { _ = b.Conn.CloseDeadline(time.Now().Add(time.Second)) }
func (b *Broker) Publish(ctx context.Context, exchange, key string, message amqp.Publishing) error {
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	select {
	case b.gate <- struct{}{}:
		defer func() { <-b.gate }()
	case <-short.Done():
		return short.Err()
	}
	message.DeliveryMode = amqp.Persistent
	deadline, _ := short.Deadline()
	_ = b.socket.SetWriteDeadline(deadline)
	defer b.socket.SetWriteDeadline(time.Time{})
	confirmation, e := b.pub.PublishWithDeferredConfirmWithContext(short, exchange, key, true, false, message)
	if e != nil {
		b.Close()
		return e
	}
	ack, e := confirmation.WaitContext(short)
	if e != nil || !ack {
		b.Close()
		return errors.New("publish not confirmed")
	}
	select {
	case <-b.returns:
		return errors.New("publish unroutable")
	default:
		return nil
	}
}
func (b *Broker) Event(ctx context.Context, kind, id string, body []byte) error {
	return b.Publish(ctx, b.Prefix+".events", kind, amqp.Publishing{ContentType: "application/json", MessageId: id, Body: body})
}
func (b *Broker) DialAddress() net.Addr { return b.Conn.RemoteAddr() }
