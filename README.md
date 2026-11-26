# shortlink

一个用 Go 标准库实现的短链接服务，重点是**并发安全**和**可测试性**，不依赖第三方框架。

## 功能

- `POST /shorten`：提交长链接，返回短码
- `GET /:code`：按短码 302 跳转到原始链接
- `GET /hello`：健康检查

## 快速开始

```bash
go run .
# 服务启动，监听 :8080
```

```bash
# 生成短码
curl -X POST http://127.0.0.1:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/some/long/path"}'
# {"code":"1"}

# 跳转
curl -i http://127.0.0.1:8080/1
# HTTP/1.1 302 Found
# Location: https://example.com/some/long/path
```

## 测试

```bash
go test ./...                  # 单元测试
go test -race ./...            # 竞态检测
go test -race -count=50 ./...  # 重复跑，降低竞态漏检概率
go vet ./...
```

测试覆盖：

- `TestHandleShorten`：正常创建
- `TestHandleRedirect`：302 与 `Location` 正确
- `TestConcurrentShorten`：20 个并发请求
- `TestConcurrentUniqueCodes`：100 个并发请求必须得到 100 个互不相同的短码

## 设计要点

### 为什么需要锁

第一版用普通 map + `counter++` 发号。并发压测下 `go test -race` 报 `DATA RACE`：

```
Read at 0x... by goroutine 15:
  main.generateCode()  main.go:17     ← counter++
Previous write at 0x... by goroutine 11:
  main.generateCode()  main.go:17
```

两个根因：

1. `counter++` 是「读 → 加 → 写」三步，不是原子操作，会丢更新
2. Go 的 `map` 不支持并发写，严重时直接 `fatal error: concurrent map writes`

### 修复方式

用一把 `sync.RWMutex` 把「取号 + 入库」收成一个逻辑上的原子操作：

```go
mu.Lock()
code := generateCode()
codeToURL[code] = req.URL
mu.Unlock()
```

读路径用 `RLock`，并且**在读完后立刻释放**，不把 `http.Redirect` 这类 I/O 放进临界区。

选 `RWMutex` 而不是 `sync.Map`，是因为 `sync.Map` 只保护 map，计数器仍要额外用 `atomic`，两套机制容易漏加锁。

### 为什么还要唯一性测试

`-race` 只能证明「没有数据竞争」，不能证明业务正确。两个请求串行地拿到同一个短码时，`-race` 不会报错，但链接已经写错了。所以补了一条断言：100 个并发请求必须产生 100 个不同短码。

## 已知限制

- 数据存在内存里，进程重启即丢失
- 短码是自增数字，可被枚举，且会暴露创建量
- 没有过期时间和容量上限
- `ListenAndServe` 的错误未处理
- 没有优雅退出

## Roadmap

- [ ] MySQL 持久化：`long_url` 唯一索引 + upsert 实现幂等
- [ ] Redis 读缓存与过期策略，评估缓存一致性
- [ ] 随机 base62 短码 + 碰撞重试
- [ ] 链接过期（TTL）与访问统计
- [ ] 限流与优雅退出（`context` + `http.Server.Shutdown`）
- [ ] 压测与基准测试（`go test -bench`）
- [ ] Dockerfile + GitHub Actions
