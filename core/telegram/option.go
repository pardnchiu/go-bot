package telegram

import (
	"net/http"
	"time"
)

type Option func(*options)

type options struct {
	httpClient  *http.Client
	pollTimeout time.Duration
}

func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		o.httpClient = client
	}
}

func WithPollTimeout(d time.Duration) Option {
	return func(o *options) {
		o.pollTimeout = d
	}
}
