.PHONY: test race lint

test:
	go test ./...

race:
	go test -race ./...

lint:
	golangci-lint run ./...
