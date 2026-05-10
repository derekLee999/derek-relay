package config

import (
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListen       = ":18080"
	defaultPollTimeout  = 25 * time.Second
	defaultMessageTTL   = 10 * time.Minute
	defaultQueueSize    = 100
	defaultMaxBodyBytes = 32 * 1024
)

type Config struct {
	Listen       string
	Secret       string
	PollTimeout  time.Duration
	MessageTTL   time.Duration
	QueueSize    int
	MaxBodyBytes int64
}

func Load() (Config, error) {
	cfg := Config{
		Listen:       envString("RELAY_LISTEN", defaultListen),
		Secret:       strings.TrimSpace(os.Getenv("RELAY_SECRET")),
		PollTimeout:  envDuration("RELAY_POLL_TIMEOUT", defaultPollTimeout),
		MessageTTL:   envDuration("RELAY_MESSAGE_TTL", defaultMessageTTL),
		QueueSize:    envInt("RELAY_QUEUE_SIZE", defaultQueueSize),
		MaxBodyBytes: int64(envInt("RELAY_MAX_BODY_BYTES", defaultMaxBodyBytes)),
	}

	flag.StringVar(&cfg.Listen, "listen", cfg.Listen, "HTTP listen address, for example :18080")
	flag.StringVar(&cfg.Secret, "secret", cfg.Secret, "shared relay secret")
	flag.DurationVar(&cfg.PollTimeout, "poll-timeout", cfg.PollTimeout, "long-poll wait timeout")
	flag.DurationVar(&cfg.MessageTTL, "message-ttl", cfg.MessageTTL, "message retention duration")
	flag.IntVar(&cfg.QueueSize, "queue-size", cfg.QueueSize, "messages kept per client")
	flag.Int64Var(&cfg.MaxBodyBytes, "max-body-bytes", cfg.MaxBodyBytes, "maximum POST body size")
	flag.Parse()

	cfg.Secret = strings.TrimSpace(cfg.Secret)
	if cfg.Secret == "" {
		return Config{}, errors.New("RELAY_SECRET or -secret is required")
	}
	if cfg.QueueSize < 1 {
		cfg.QueueSize = defaultQueueSize
	}
	if cfg.PollTimeout < time.Second {
		cfg.PollTimeout = defaultPollTimeout
	}
	if cfg.MessageTTL < time.Minute {
		cfg.MessageTTL = defaultMessageTTL
	}
	if cfg.MaxBodyBytes < 1024 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}

	return cfg, nil
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return number
}
