tidy:
	go mod tidy
test: tidy
	go test ./... -v
	go test -v -bench=. ./... -benchmem

run:
	go run . server --config misc/waitingroom/waitingroom.toml --log-level debug

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/waitingroom .

ci: lint test

lint: devdeps
	@staticcheck ./...


devdeps:
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
