MIGRATE_IMAGE := migrate/migrate:v4.19.1
MIGRATIONS_PATH := $(CURDIR)/migrations

.PHONY: migrate-up migrate-down migrate-version

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
