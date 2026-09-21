SHELL := /bin/sh

ARTIFACT_DIR := $(CURDIR)/artifacts
BACKEND_COVERAGE := $(ARTIFACT_DIR)/coverage/backend.out
UI_COVERAGE_DIR := $(ARTIFACT_DIR)/coverage/ui
TEST_DATABASE_URL ?= postgres://stratum_test:stratum_test_password@127.0.0.1:55432/stratum_test?sslmode=disable
MIGRATION_DATABASE_URL ?= $(DATABASE_URL)

.PHONY: test test-backend test-backend-unit test-backend-integration test-ui test-ui-unit test-e2e test-db-up test-db-down migrate-status migrate-check migrate-up clean-test-artifacts

test: test-backend test-ui

test-backend: test-backend-unit test-backend-integration

test-backend-unit:
	@mkdir -p "$(dir $(BACKEND_COVERAGE))"
	cd backend && GOCACHE="$(CURDIR)/backend/.gocache" go test -race -covermode=atomic -coverprofile="$(BACKEND_COVERAGE)" ./...
	cd backend && GOCACHE="$(CURDIR)/backend/.gocache" go tool cover -func="$(BACKEND_COVERAGE)" | tail -1

test-db-up:
	docker compose -f docker-compose.test.yml up -d --wait postgres-test

test-db-down:
	docker compose -f docker-compose.test.yml down --volumes --remove-orphans

test-backend-integration: test-db-up
	@trap '$(MAKE) -C "$(CURDIR)" test-db-down' EXIT; \
	cd backend && GOCACHE="$(CURDIR)/backend/.gocache" TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -tags=integration ./internal/store ./internal/httpapi

migrate-status:
	@test -n "$(MIGRATION_DATABASE_URL)" || (echo "MIGRATION_DATABASE_URL or DATABASE_URL is required" && exit 1)
	@cd backend && GOCACHE="$(CURDIR)/backend/.gocache" MIGRATION_DATABASE_URL="$(MIGRATION_DATABASE_URL)" go run ./cmd/migrate status

migrate-check:
	@test -n "$(MIGRATION_DATABASE_URL)" || (echo "MIGRATION_DATABASE_URL or DATABASE_URL is required" && exit 1)
	@cd backend && GOCACHE="$(CURDIR)/backend/.gocache" MIGRATION_DATABASE_URL="$(MIGRATION_DATABASE_URL)" go run ./cmd/migrate check

migrate-up:
	@test -n "$(MIGRATION_DATABASE_URL)" || (echo "MIGRATION_DATABASE_URL or DATABASE_URL is required" && exit 1)
	@cd backend && GOCACHE="$(CURDIR)/backend/.gocache" MIGRATION_DATABASE_URL="$(MIGRATION_DATABASE_URL)" go run ./cmd/migrate up

test-ui: test-ui-unit
	cd ui && npm run lint
	cd ui && npm run build

test-ui-unit:
	@mkdir -p "$(UI_COVERAGE_DIR)"
	cd ui && npm run test:coverage -- --coverage.reportsDirectory="$(UI_COVERAGE_DIR)"

test-e2e:
	cd ui && npm run test:e2e

clean-test-artifacts:
	rm -rf "$(ARTIFACT_DIR)"
