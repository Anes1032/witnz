.PHONY: build test clean release install docker-build docker-push test-append-only test-verify test-election-timeout test-leadership-transfer test-node-restart test-integration

VERSION ?= 0.2.0
DOCKER_IMAGE ?= witnz/witnz

build:
	go build -o witnz ./cmd/witnz

test:
	go test ./internal/... -v

test-coverage:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

test-append-only:
	./scripts/test-append-only.sh

test-verify:
	./scripts/test-verify.sh

test-election-timeout:
	./scripts/test-election-timeout.sh

test-leadership-transfer:
	./scripts/test-leadership-transfer.sh

test-node-restart:
	./scripts/test-node-restart.sh

test-integration:
	./scripts/test-integration.sh

clean:
	rm -f witnz
	rm -rf dist/
	rm -f coverage.out coverage.html

release:
	./scripts/build-release.sh $(VERSION)

install:
	go install ./cmd/witnz

docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

dev:
	docker-compose up -d

dev-down:
	docker-compose down

dev-logs:
	docker-compose logs -f

help:
	@echo "Witnz Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build                     - Build single binary"
	@echo "  test                      - Run unit tests"
	@echo "  test-coverage             - Generate test coverage report"
	@echo "  test-all                  - Run all integration tests (requires Docker)"
	@echo "  test-append-only          - Test append-only mode (requires Docker)"
	@echo "  test-verify               - Test hash chain verification (requires Docker)"
	@echo "  test-election-timeout     - Test Raft leader election (requires Docker)"
	@echo "  test-follower-compromise  - Test follower compromise detection (requires Docker)"
	@echo "  clean                     - Remove build artifacts"
	@echo "  release                   - Build for all platforms (VERSION=0.2.0)"
	@echo "  install                   - Install to GOPATH/bin"
	@echo "  docker-build              - Build Docker image"
	@echo "  docker-push               - Push Docker image to registry"
	@echo "  dev                       - Start development environment"
	@echo "  dev-down                  - Stop development environment"
	@echo "  dev-logs                  - View development logs"
