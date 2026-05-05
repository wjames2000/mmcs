.PHONY: frontend desktop dev test clean

# 构建前端
frontend:
	cd frontend && npm run build && mkdir -p ../cmd/desktop/frontend/dist && cp -r dist/* ../cmd/desktop/frontend/dist/

# 构建 Wails 桌面应用
desktop: frontend
	cd cmd/desktop && wails build -clean -skipbindings

# 开发模式（需要先启动 docker）
dev:
	docker compose up -d postgres redis
	cd frontend && npm run dev &
	cd cmd/desktop && wails dev

# 运行所有测试
test:
	go test -race -count=1 ./...

# 全量构建检查
check: test
	go build ./...
	go vet ./...
	cd frontend && npx tsc --noEmit && npm run build

# 清理构建产物
clean:
	rm -rf cmd/desktop/build
	rm -rf cmd/desktop/frontend/dist
	rm -rf frontend/dist
