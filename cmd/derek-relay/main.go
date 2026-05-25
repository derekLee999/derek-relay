package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"derek-relay/internal/config"
	"derek-relay/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	relay := server.New(cfg)
	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           relay.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf(
		"derek-relay starting listen=%s poll_timeout=%s message_ttl=%s queue_size=%d max_body_bytes=%d",
		cfg.Listen,
		cfg.PollTimeout,
		cfg.MessageTTL,
		cfg.QueueSize,
		cfg.MaxBodyBytes,
	)

	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("shutdown requested signal=%s", sig)
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("http server failed: %v", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Print("derek-relay stopped")
}
