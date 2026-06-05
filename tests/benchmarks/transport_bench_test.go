package benchmarks

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func BenchmarkHTTP_NewTransportPerRequest(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	b.ResetTimer()
	var mu sync.Mutex
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			func() {
				client := &http.Client{
					Transport: &http.Transport{
						MaxIdleConnsPerHost: 2,
						DisableKeepAlives:   true,
					},
				}
				resp, err := client.Get(ts.URL)
				if err != nil {
					mu.Lock()
					b.Log("request error:", err)
					mu.Unlock()
					return
				}
				resp.Body.Close()
			}()
		}
	})
}

func BenchmarkHTTP_SharedTransport(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(ts.URL)
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}
