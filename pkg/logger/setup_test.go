package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestVectorHookFatalPublishesSynchronously(t *testing.T) {
	gotRequest := make(chan struct{}, 1)
	allowResponse := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotRequest <- struct{}{}:
		default:
		}
		<-allowResponse
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	hook := newVectorHook(server.URL, 1)
	defer hook.Close()

	base := logrus.New()
	base.SetFormatter(&logrus.JSONFormatter{})
	entry := logrus.NewEntry(base).WithField("component", "test")
	entry.Level = logrus.FatalLevel
	entry.Message = "boom"
	entry.Time = time.Now().UTC()

	done := make(chan error, 1)
	go func() {
		done <- hook.Fire(entry)
	}()

	select {
	case <-gotRequest:
	case <-time.After(time.Second):
		t.Fatal("fatal log was not published synchronously")
	}

	select {
	case <-done:
		t.Fatal("fatal hook returned before publish completed")
	default:
	}

	close(allowResponse)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("fire returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fatal publish did not finish")
	}
}
