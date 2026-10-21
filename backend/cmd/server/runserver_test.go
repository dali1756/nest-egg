package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(s string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, s)
}

func (e *events) snapshot() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.list)
}

func slowJob(ev *events) func(context.Context) {
	return func(ctx context.Context) {
		<-ctx.Done()
		ev.add("job cancelled")
		time.Sleep(50 * time.Millisecond)
		ev.add("job finished")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestRunServerStopsHTTPBeforeJobs(t *testing.T) {
	ev := &events{}
	entered, release := make(chan struct{}), make(chan struct{})
	srv := newServer(freeAddr(t), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
	}))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	served := make(chan error, 1)
	go func() {
		err := runServer(ctx, srv, slowJob(ev))
		ev.add("runServer returned")
		served <- err
	}()

	go func() {
		for {
			resp, err := http.Get("http://" + srv.Addr)
			if err == nil {
				resp.Body.Close()
				return
			}
			select {
			case <-entered:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach the handler")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
	if got := ev.snapshot(); len(got) != 0 {
		t.Fatalf("events while HTTP is still draining = %v, want none (jobs stop after HTTP)", got)
	}
	close(release)

	select {
	case err := <-served:
		if err != nil {
			t.Errorf("runServer = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServer did not return")
	}
	if got, want := ev.snapshot(), []string{"job cancelled", "job finished", "runServer returned"}; !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}

func TestRunServerStopsJobsWhenHTTPFailsToStart(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	ev := &events{}

	err = runServer(t.Context(), newServer(busy.Addr().String(), http.NotFoundHandler()), slowJob(ev))
	ev.add("runServer returned")

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Errorf("runServer = %v, want the listen error", err)
	}
	if got, want := ev.snapshot(), []string{"job cancelled", "job finished", "runServer returned"}; !slices.Equal(got, want) {
		t.Errorf("events = %v, want %v", got, want)
	}
}
