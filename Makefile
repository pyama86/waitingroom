tidy:
	go mod tidy
test: tidy
	go test ./... -v

run:
	go run .
