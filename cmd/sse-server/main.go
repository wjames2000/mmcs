// sse-server 已合并至 api-server
// SSE 流式推送由 api-server 在 /api/v1/sessions/{id}/stream 路径提供
// 不再需要独立的 SSE HTTP 服务
//
// 如果需要独立扩容 SSE 连接，可在此文件基础上恢复独立部署，
// 通过共享 Redis 实现跨进程 SSE Hub。
package main

func main() {}
