tidy:
	go mod tidy
test: tidy
	go test ./... -v
	go test -v -bench=. ./... -benchmem

run: swag
	go run . server --config misc/waitingroom/waitingroom.toml --log-level debug

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/waitingroom .

ci: lint test

lint: devdeps
	@staticcheck ./...

viron:
	docker build -t viron --file Dockerfile.viron .
	docker run --rm -it viron --net host

devdeps:
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest

swag:
	swag i
