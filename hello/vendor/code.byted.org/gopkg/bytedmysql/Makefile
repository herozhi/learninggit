.PHONY: lint
lint: deps
	@GO111MODULE=on go fmt ./...
	@GO111MODULE=on find . -not -path "./vendor/*" -type f -name "*.go" -print0|xargs -0 goimports -local code.byted.org/gopkg/bytedmysql -w
	@GO111MODULE=on go mod tidy
.PHONY: vet
vet:
	@GO111MODULE=on go vet ./...
.PHONY: deps
deps:
	@GO111MODULE=on hash goimports > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get golang.org/x/tools/cmd/goimports; \
	fi
.PHONY: test
test:
	@GO111MODULE=on CONSUL_HTTP_HOST='10.225.108.122' go test ./...
