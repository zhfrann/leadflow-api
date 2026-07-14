MIGRATE_IMAGE := migrate/migrate:v4.19.1
MIGRATIONS_PATH := $(CURDIR)/migrations

POSTGRES_USER ?= leadflow
TEST_POSTGRES_DB ?= leadflow_test
TEST_DATABASE_URL ?= postgres://leadflow:leadflow_dev_password@localhost:5432/$(TEST_POSTGRES_DB)?sslmode=disable

.PHONY: migrate-up migrate-down migrate-version test-db-create test-migrate-up test-integration test-integration-race

migrate-up:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	docker run --rm \
		--network host \
		-v "$(MIGRATIONS_PATH):/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL)" \
		up

migrate-down:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	docker run --rm \
		--network host \
		-v "$(MIGRATIONS_PATH):/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL)" \
		down 1

migrate-version:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	docker run --rm \
		--network host \
		-v "$(MIGRATIONS_PATH):/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL)" \
		version

test-db-create:
	@docker compose exec -T postgres \
		psql -U "$(POSTGRES_USER)" -d postgres -tAc \
		"SELECT 1 FROM pg_database WHERE datname = '$(TEST_POSTGRES_DB)'" \
		| grep -q 1 || \
		docker compose exec -T postgres \
		createdb -U "$(POSTGRES_USER)" "$(TEST_POSTGRES_DB)"

test-migrate-up: test-db-create
	docker run --rm \
		--network host \
		-v "$(MIGRATIONS_PATH):/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(TEST_DATABASE_URL)" \
		up

test-integration: test-migrate-up
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
		go test -tags=integration -count=1 ./tests/integration

test-integration-race: test-migrate-up
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
		go test -race -tags=integration -count=1 ./tests/integration
