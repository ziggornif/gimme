audit:
	gosec ./..

build:
	go build -ldflags "-w -s" -o gimme


tests:
	go test  ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out

test-coverage: tests coverage