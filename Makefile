tidy:
	go mod tidy
test: tidy
	go test ./... -v
	go test -v -bench=. ./... -benchmem

run:
	go run . server
