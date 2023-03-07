tidy:
	go mod tidy
test: tidy
	go test ./... -v
	go test -v -bench=. ./... -benchmem

run: swag
	go run . server --config misc/waitingroom/waitingroom.toml --log-level debug --dev

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/waitingroom .

ci: lint test

lint: devdeps
	@staticcheck ./...

.PHONY: viron
viron:
	docker build -t viron viron
	docker run -p 9090:9090 --rm -it viron

devdeps:
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest

swag:
	swag i
