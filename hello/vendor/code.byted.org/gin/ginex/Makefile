all: fmt test

fmt:
	find ./ -name "*.go" | xargs goimports -w
	find ./ -name "*.go" | xargs gofmt -w

test:
	go test -v ./...

