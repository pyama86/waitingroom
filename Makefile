tidy:
	go mod tidy
test: tidy
	go test ./... -v
	go test -v -bench=. ./... -benchmem

run:
	go run . server


ci: lint test

lint: devdeps
	@staticcheck ./...


devdeps:
	@which staticcheck > /dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
