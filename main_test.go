package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHandleShorten(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"http://douyin.com/follow"}`))
	handleShorten(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

}

func TestHandleRedirect(t *testing.T) {
	codeToURL["abc"] = "http://douyin.com/follow"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/abc", nil)
	handleRedirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("want 302 ,got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "http://douyin.com/follow" {
		t.Fatalf("bad Location: %s", loc)
	}
}

func TestConcurrentShorten(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/shorten",
				strings.NewReader(`{"url":"http://douyin.com/follow"}`))
			handleShorten(rec, req)
		}()

	}
	wg.Wait()
}

// TestConcurrentUniqueCodes 验证业务语义：并发请求必须拿到互不相同的短码。
// 这比"没有 DATA RACE"更接近真实需求——不崩不代表正确。
func TestConcurrentUniqueCodes(t *testing.T) {
	const n = 100
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		codes = make(map[string]struct{}, n)
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/shorten",
				strings.NewReader(`{"url":"http://douyin.com/follow"}`))
			handleShorten(rec, req)

			var resp struct {
				Code string `json:"code"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				return
			}
			mu.Lock()
			codes[resp.Code] = struct{}{}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(codes) != n {
		t.Fatalf("want %d unique codes, got %d", n, len(codes))
	}
}
