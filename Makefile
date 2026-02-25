.PHONY: audit
audit:
	gosec ./..

.PHONY: build
build:
	go build -ldflags "-w -s" -o gimme ./cmd/server/main.go

.PHONY: release
release:
	rm -rf dist
	mkdir dist
	env GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o gimme ./cmd/server/main.go && upx --best ./gimme
	cp -R gimme docs templates ./dist

.PHONY: test
test: garage-start
	@KEY_ID=$$(docker exec $(GARAGE_CONTAINER) /garage key info --show-secret gimme-key | grep 'Key ID'     | awk '{print $$3}') && \
	 SECRET=$$(docker exec $(GARAGE_CONTAINER) /garage key info --show-secret gimme-key | grep 'Secret key' | awk '{print $$3}') && \
	 TEST_S3_URL=localhost:3900 TEST_S3_KEY="$$KEY_ID" TEST_S3_SECRET="$$SECRET" \
	 TEST_S3_BUCKET=gimme TEST_S3_LOCATION=garage \
	 go test ./... -coverprofile=coverage.out; \
	 grep -v "github.com/gimme-cdn/gimme/test/" coverage.out > coverage.tmp && mv coverage.tmp coverage.out; \
	 $(MAKE) garage-stop

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
	@echo "Done."

.PHONY: test-integration
test-integration: test
