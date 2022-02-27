MAIN=main.go

audit:
	gosec ./..

build:
	go build -ldflags "-w -s" -o gimme $(MAIN)

test:
	go test  ./... -coverprofile=coverage.out
coverage:
	 go tool cover -html=coverage.out

test-coverage: test coverage