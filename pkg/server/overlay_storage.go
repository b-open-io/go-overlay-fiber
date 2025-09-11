package server

import (
	"context"
	"net/url"
	"strings"

	"github.com/b-open-io/overlay/pubsub"
)

// NoOpPublisher implements overlay's publish.Publisher interface with no-op behavior
type NoOpPublisher struct{}

func (p *NoOpPublisher) Subscribe(ctx context.Context, topics []string) (<-chan pubsub.Event, error) {
	//TODO implement me
	panic("implement me")
}

func (p *NoOpPublisher) Unsubscribe(topics []string) error {
	//TODO implement me
	panic("implement me")
}

func (p *NoOpPublisher) Stop() error {
	//TODO implement me
	panic("implement me")
}

func (p *NoOpPublisher) Close() error {
	//TODO implement me
	panic("implement me")
}

func (p *NoOpPublisher) Publish(ctx context.Context, topic string, data string, score ...float64) error {
	// No-op implementation
	_ = ctx
	_ = topic
	_ = data
	return nil
}

// getStorageType returns a human-readable storage type from URL
func getStorageType(storageURL string) string {
	if storageURL == "" {
		return "SQL"
	}

	if strings.HasPrefix(storageURL, "mongodb://") {
		return "MongoDB"
	}
	if strings.HasPrefix(storageURL, "sqlite://") || strings.HasSuffix(storageURL, ".db") {
		return "SQLite"
	}
	if strings.HasSuffix(storageURL, "/") {
		return "Filesystem"
	}

	// Try parsing as URL
	if u, err := url.Parse(storageURL); err == nil && u.Scheme != "" {
		return strings.ToUpper(u.Scheme)
	}

	return "Filesystem"
}
