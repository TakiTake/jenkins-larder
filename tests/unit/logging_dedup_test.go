package unit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/server"
)

func TestLoggingMiddleware(t *testing.T) {
	t.Run("passes through to handler", func(t *testing.T) {
		var called bool
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		handler := server.LoggingMiddleware(inner)
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if !called {
			t.Error("inner handler was not called")
		}
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})

	t.Run("captures non-200 status code", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		handler := server.LoggingMiddleware(inner)
		req := httptest.NewRequest("GET", "/missing", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rr.Code)
		}
	})
}

func TestDeduplicationManager(t *testing.T) {
	t.Run("deduplicates concurrent calls", func(t *testing.T) {
		dm := server.NewDeduplicationManager()
		var calls int32

		fn := func() (interface{}, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(50 * time.Millisecond)
			return "result", nil
		}

		// Launch concurrent calls
		done := make(chan interface{}, 3)
		for i := 0; i < 3; i++ {
			go func() {
				result, err := dm.Do(context.Background(), "key1", fn)
				if err != nil {
					done <- err
				} else {
					done <- result
				}
			}()
		}

		for i := 0; i < 3; i++ {
			r := <-done
			if r != "result" {
				t.Errorf("unexpected result: %v", r)
			}
		}

		if c := atomic.LoadInt32(&calls); c != 1 {
			t.Errorf("expected 1 call, got %d", c)
		}
	})

	t.Run("propagates errors", func(t *testing.T) {
		dm := server.NewDeduplicationManager()
		expectedErr := errors.New("test error")

		_, err := dm.Do(context.Background(), "err-key", func() (interface{}, error) {
			return nil, expectedErr
		})

		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("forget allows re-execution", func(t *testing.T) {
		dm := server.NewDeduplicationManager()
		var calls int32

		fn := func() (interface{}, error) {
			return atomic.AddInt32(&calls, 1), nil
		}

		result1, err := dm.Do(context.Background(), "forget-key", fn)
		if err != nil {
			t.Fatal(err)
		}

		dm.Forget("forget-key")

		result2, err := dm.Do(context.Background(), "forget-key", fn)
		if err != nil {
			t.Fatal(err)
		}

		if result1 == result2 {
			t.Error("expected different results after Forget")
		}
	})
}
