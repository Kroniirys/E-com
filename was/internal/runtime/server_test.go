package runtime

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownDrainsInFlightRequest(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-release; w.WriteHeader(200) })}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- Serve(ctx, srv, listener, time.Second) }()
	requestResult := make(chan error, 1)
	go func() {
		client := http.Client{Timeout: 2 * time.Second}
		res, e := client.Get("http://" + listener.Addr().String())
		if e == nil {
			_ = res.Body.Close()
		}
		requestResult <- e
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request never entered")
	}
	cancel()
	select {
	case <-result:
		t.Fatal("shutdown abandoned in-flight request")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-requestResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request blocked")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
}
