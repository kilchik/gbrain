BUILD_COMMIT := $(shell git log --format="%H" -n 1)

gbrain:
	go build -o target/ -ldflags="-X 'main.BuildCommit=$(BUILD_COMMIT)'" ./...


test:
	go test ./...

check:
	golangci-lint run -c golangci-lint.yaml

