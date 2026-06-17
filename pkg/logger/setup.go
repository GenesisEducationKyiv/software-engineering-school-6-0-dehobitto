package logger

import (
	"context"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultVectorBufferSize = 1000
	vectorRequestTimeout    = 2 * time.Second
)

func Configure(level string, vectorEnabled bool, vectorURL string, logFiles ...string) (func(), error) {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	parsed, err := logrus.ParseLevel(level)
	if err != nil {
		return func() {}, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	logrus.SetLevel(parsed)

	var logFile *os.File
	if len(logFiles) > 0 && logFiles[0] != "" {
		file, err := os.OpenFile(logFiles[0], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return func() {}, fmt.Errorf("open log file: %w", err)
		}
		logFile = file
		logrus.SetOutput(io.MultiWriter(os.Stdout, file))
	} else {
		logrus.SetOutput(os.Stdout)
	}

	cleanupFile := func() {
		if logFile != nil {
			if err := logFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "close log file: %v\n", err)
			}
		}
	}

	if !vectorEnabled {
		return cleanupFile, nil
	}
	if vectorURL == "" {
		vectorURL = "http://vector:8686"
	}
	hook := newVectorHook(vectorURL, defaultVectorBufferSize)
	logrus.AddHook(hook)
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := hook.CloseWithContext(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "vector log hook: close failed: %v\n", err)
		}
		cleanupFile()
	}, nil
}

type vectorHook struct {
	url     string
	client  *http.Client
	entries chan []byte
	closeCtx context.Context
	wg      sync.WaitGroup
	closed  bool
	mu      sync.RWMutex
}

func newVectorHook(url string, bufferSize int) *vectorHook {
	hook := &vectorHook{
		url:     url,
		client:  &http.Client{Timeout: vectorRequestTimeout},
		entries: make(chan []byte, bufferSize),
		closeCtx: context.Background(),
	}
	hook.wg.Add(1)
	go hook.publishLoop()
	return hook
}

func (h *vectorHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *vectorHook) Fire(entry *logrus.Entry) error {
	body, err := entry.Bytes()
	if err != nil {
		return err
	}

	if entry.Level == logrus.FatalLevel {
		return h.publish(body)
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return nil
	}
	select {
	case h.entries <- body:
	default:
		fmt.Fprintln(os.Stderr, "vector log hook: buffer full, dropping log entry")
	}
	return nil
}

func (h *vectorHook) publishLoop() {
	defer h.wg.Done()
	for body := range h.entries {
		_ = h.publish(h.publishContext(), body)
	}
}

func (h *vectorHook) publishContext() context.Context {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closeCtx
}

func (h *vectorHook) publish(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "vector log hook: build request failed: %v\n", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "vector log hook: publish failed: %v\n", err)
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(os.Stderr, "vector log hook: publish returned status %d\n", resp.StatusCode)
		return fmt.Errorf("publish returned status %d", resp.StatusCode)
	}
	return nil
}

func (h *vectorHook) CloseWithContext(ctx context.Context) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.closeCtx = ctx
	close(h.entries)
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *vectorHook) Close() {
	_ = h.CloseWithContext(context.Background())
}
