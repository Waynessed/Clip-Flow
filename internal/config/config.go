package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

func Env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func Seconds(key string, fallback int) time.Duration {
	n, err := strconv.Atoi(Env(key, strconv.Itoa(fallback)))
	if err != nil || n < 1 {
		panic("invalid " + key)
	}
	return time.Duration(n) * time.Second
}
func Logging() { slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil))) }
