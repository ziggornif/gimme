.PHONY: audit
audit:
	gosec ./..

.PHONY: build
build:
	go build -ldflags "-w -s" -o gimme

.PHONY: test
test:
	go test  ./... -coverprofile=coverage.out

.PHONY: coverage
coverage:
	go tool cover -html=coverage.out

.PHONY: test_coverage
test_coverage: test coverage

.PHONY: watch
watch:
	air -c .air.toml