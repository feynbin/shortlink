package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// 真希望雨能下不停
var (
	// mu 保护 codeToURL 和 counter。读用 RLock，写用 Lock。
	mu        sync.RWMutex
	codeToURL = map[string]string{}
	counter   int
)

// generateCode 负责发号：唯一、递增。
// 注意：调用前必须已经持有 mu 的写锁。
func generateCode() string {
	counter++
	return strconv.Itoa(counter)
}
func handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求体格式错误", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url 不能为空", http.StatusBadRequest)
		return
	}

	// 取号和写 map 必须是一个整体：否则两个请求可能拿到同一个短码。
	mu.Lock()
	code := generateCode()
	codeToURL[code] = req.URL
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"code": code})
}
func handleRedirect(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/")
	if code == "" {
		http.Error(w, "缺少短码", http.StatusBadRequest)
		return
	}

	// 只在读 map 的瞬间持锁，别把 http.Redirect 也包在锁里。
	mu.RLock()
	url, ok := codeToURL[code]
	mu.RUnlock()
	if !ok {
		http.Error(w, "短码不存在", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
func main() {
	// 我爱上给我 勇气的ranie love

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	})
	http.HandleFunc("/shorten", handleShorten)
	http.HandleFunc("/", handleRedirect)
	fmt.Println("服务启动，监听 :8080")
	http.ListenAndServe(":8080", nil)
}
