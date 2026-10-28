package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestRunHTTPDrainsInFlightRequestOnShutdown proves a stop signal lets an active request finish before exit.
func TestRunHTTPDrainsInFlightRequestOnShutdown(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	server := newHTTPServer("127.0.0.1:18089", handler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	// The test owns this goroutine; it ends when runHTTP returns after shutdown.
	go func() { done <- runHTTP(ctx, server) }()

	status := make(chan int, 1)
	// The test owns this client goroutine; it sends one status (or 0 on error) and exits.
	go func() {
		var response *http.Response
		var err error
		for i := 0; i < 50; i++ {
			if response, err = http.Get("http://127.0.0.1:18089/"); err == nil {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err != nil {
			status <- 0
			return
		}
		response.Body.Close()
		status <- response.StatusCode
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request never reached the handler")
	}
	cancel()
	close(release)
	if got := <-status; got != http.StatusNoContent {
		t.Fatalf("in-flight request status = %d, want 204", got)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runHTTP returned %v after graceful shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after shutdown")
	}
	if server.ReadHeaderTimeout == 0 || server.WriteTimeout == 0 {
		t.Fatal("server must bound slow clients")
	}
}
