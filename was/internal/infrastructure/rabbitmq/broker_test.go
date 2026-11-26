package rabbitmq

import (
	"net"
	"testing"
	"time"
)

func TestHandshakeTimeout(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		c, e := listener.Accept()
		if e == nil {
			defer c.Close()
			<-done
		}
	}()
	defer close(done)
	start := time.Now()
	b, e := Open("amqp://guest:guest@"+listener.Addr().String()+"/", "handshake_test")
	if e == nil {
		b.Close()
		t.Fatal("silent peer accepted")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("handshake exceeded timeout")
	}
}
