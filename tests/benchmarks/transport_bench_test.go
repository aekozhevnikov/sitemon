package benchmarks

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var sharedServer *httptest.Server

func init() {
	sharedServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func BenchmarkHTTP_NewTransportPerRequest(b *testing.B) {
	ts := sharedServer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
				MaxIdleConns:      1,
			},
		}
		resp, err := client.Get(ts.URL)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkHTTP_SharedTransport(b *testing.B) {
	ts := sharedServer

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(ts.URL)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}