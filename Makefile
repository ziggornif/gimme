.PHONY: audit
audit:
	gosec ./...

.PHONY: build
build:
	go build -ldflags "-w -s" -o gimme ./cmd/server/main.go

# GOOS/GOARCH can be overridden on the command line or via environment variables.
# When invoked from the Dockerfile, they are set via ARG/ENV so the build
# automatically targets the correct platform (e.g. linux/arm64 for multi-arch).
GOOS  ?= linux
GOARCH ?= amd64

.PHONY: release
release:
	rm -rf dist
	mkdir dist
	env GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s" -o gimme ./cmd/server/main.go && upx --fast ./gimme
	cp -R gimme docs templates assets ./dist

# release-fast skips UPX compression — useful for quick local Docker builds
.PHONY: release-fast
release-fast:
	rm -rf dist
	mkdir dist
	env GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-w -s" -o gimme ./cmd/server/main.go
	cp -R gimme docs templates assets ./dist

.PHONY: test
test: garage-start
	@KEY_ID=$$(docker exec $(GARAGE_CONTAINER) /garage key info --show-secret gimme-key | grep 'Key ID'     | awk '{print $$3}') && \
	 SECRET=$$(docker exec $(GARAGE_CONTAINER) /garage key info --show-secret gimme-key | grep 'Secret key' | awk '{print $$3}') && \
	 TEST_S3_URL=localhost:3900 TEST_S3_KEY="$$KEY_ID" TEST_S3_SECRET="$$SECRET" \
	 TEST_S3_BUCKET=gimme TEST_S3_LOCATION=garage \
	 go test ./... -coverprofile=coverage.out; \
	 TEST_EXIT=$$?; \
	 grep -v "github.com/gimme-cdn/gimme/test/" coverage.out > coverage.tmp && mv coverage.tmp coverage.out; \
	 $(MAKE) garage-stop; \
	 exit $$TEST_EXIT

.PHONY: coverage
coverage:
	go tool cover -func coverage.out

.PHONY: html_coverage
html_coverage:
	go tool cover -html=coverage.out

.PHONY: test_coverage
test_coverage: test coverage

.PHONY: test_coverage_html
test_coverage_html: test html_coverage

.PHONY: watch
watch:
	air -c .air.toml

GARAGE_VERSION ?= v1.3.1
GARAGE_CONTAINER ?= gimme-garage-test

.PHONY: garage-start
garage-start:
	@echo "Starting Garage $(GARAGE_VERSION)..."
	@docker rm -f $(GARAGE_CONTAINER) >/dev/null 2>&1 || true
	@mkdir -p /tmp/garage/{meta,data}
	@printf '%s\n' \
		'metadata_dir = "/var/lib/garage/meta"' \
		'data_dir     = "/var/lib/garage/data"' \
		'db_engine    = "sqlite"' \
		'replication_factor = 1' \
		'rpc_bind_addr   = "[::]:3901"' \
		'rpc_public_addr = "127.0.0.1:3901"' \
		'rpc_secret = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"' \
		'[s3_api]' \
		's3_region    = "garage"' \
		'api_bind_addr = "[::]:3900"' \
		'[admin]' \
		'api_bind_addr = "[::]:3903"' \
		> /tmp/garage.toml
	@docker run -d --name $(GARAGE_CONTAINER) \
		-p 3900:3900 -p 3901:3901 -p 3903:3903 \
		-v /tmp/garage.toml:/etc/garage.toml \
		-v /tmp/garage/meta:/var/lib/garage/meta \
		-v /tmp/garage/data:/var/lib/garage/data \
		dxflrs/garage:$(GARAGE_VERSION)
	@echo "Waiting for Garage to be ready..."
	@for i in $$(seq 1 15); do \
		docker exec $(GARAGE_CONTAINER) /garage status >/dev/null 2>&1 && break; \
		echo "  attempt $$i/15..."; \
		sleep 2; \
	done
	@NODE_ID=$$(docker exec $(GARAGE_CONTAINER) /garage status | grep -oP '[0-9a-f]{16}' | head -1) && \
		docker exec $(GARAGE_CONTAINER) /garage layout assign -z dc1 -c 1G "$$NODE_ID" && \
		docker exec $(GARAGE_CONTAINER) /garage layout apply --version 1 && \
		docker exec $(GARAGE_CONTAINER) /garage key create gimme-key && \
		docker exec $(GARAGE_CONTAINER) /garage bucket create gimme && \
		docker exec $(GARAGE_CONTAINER) /garage bucket allow --read --write --owner gimme --key gimme-key
	@echo "Garage ready."

.PHONY: garage-stop
garage-stop:
	@echo "Stopping Garage..."
	@docker rm -f $(GARAGE_CONTAINER) >/dev/null 2>&1 || true
	@docker run --rm -v /tmp:/tmp alpine sh -c "rm -rf /tmp/garage /tmp/garage.toml" 2>/dev/null || true
	@echo "Done."

.PHONY: test-integration
test-integration: test

DOC_PORT ?= 8000

.PHONY: docs-serve
docs-serve:
	@echo "Linking assets..."
	@rm -rf docs/site/assets
	@ln -s ../../assets docs/site/assets
	@echo "Starting doc server at http://localhost:$(DOC_PORT) (Ctrl+C to stop)"
	@trap 'echo "Unlinking assets..."; rm -f docs/site/assets; echo "Done."' EXIT INT TERM; \
	 cd docs/site && python3 -m http.server $(DOC_PORT)
