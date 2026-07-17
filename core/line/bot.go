package line

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot"
)

type Status struct {
	Running     bool
	UserID      string
	BasicID     string
	DisplayName string
}

type Bot struct {
	api       *linebot.Client
	addr      string
	path      string
	server    *http.Server
	mu        sync.Mutex
	cancel    context.CancelFunc
	ctx       context.Context
	done      chan struct{}
	running   bool
	info      *linebot.BotInfoResponse
	handlerMu sync.RWMutex
	handler   ReplyHandler
}

func New(secret, token, port string, opts ...Option) (*Bot, error) {
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if port == "" {
		return nil, fmt.Errorf("port is required")
	}

	o := options{path: "/linebot/webhook"}
	for _, opt := range opts {
		opt(&o)
	}

	api, err := linebot.New(secret, token)
	if err != nil {
		return nil, fmt.Errorf("line-bot-sdk-go New: %w", err)
	}

	return &Bot{
		api:  api,
		addr: ":" + port,
		path: o.path,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	info, err := b.api.GetBotInfo().WithContext(ctx).Do()
	if err != nil {
		return fmt.Errorf("line-bot-sdk-go GetBotInfo: %w", err)
	}

	ln, err := net.Listen("tcp", b.addr)
	if err != nil {
		return fmt.Errorf("net.Listen %s: %w", b.addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(b.path, b.webhook)
	server := &http.Server{Handler: mux}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	b.server = server
	b.cancel = cancel
	b.ctx = runCtx
	b.done = done
	b.running = true
	b.info = info

	go func() {
		defer close(done)
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Warn("line Bot.Serve",
				slog.String("err", err.Error()))
		}
	}()
	return nil
}

func (b *Bot) Close() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}

	cancel := b.cancel
	server := b.server
	done := b.done
	b.running = false
	b.cancel = nil
	b.server = nil
	b.ctx = nil
	b.done = nil
	b.mu.Unlock()

	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	err := server.Shutdown(shutCtx)
	<-done
	if err != nil {
		return fmt.Errorf("line Bot.Shutdown: %w", err)
	}
	return nil
}

func (b *Bot) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()

	status := Status{Running: b.running}
	if b.info != nil {
		status.UserID = b.info.UserID
		status.BasicID = b.info.BasicID
		status.DisplayName = b.info.DisplayName
	}
	return status
}
