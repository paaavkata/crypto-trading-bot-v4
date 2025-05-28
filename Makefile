DB_URL ?= "postgres://user:password@localhost:5432/crypto_bot_db?sslmode=disable"

migrate-create:
ifeq ($(NAME),)
	@echo "Usage: make migrate-create NAME=<migration_name>"
	@exit 1
endif
	@echo "Creating migration files for $(NAME)..."
	migrate create -ext sql -dir migrations -seq $(NAME)

migrate-up:
	@echo "Applying all up migrations..."
	migrate -database "$(DB_URL)" -path migrations up

migrate-down-one:
	@echo "Reverting last migration..."
	migrate -database "$(DB_URL)" -path migrations down 1

migrate-to-version:
ifeq ($(VERSION),)
	@echo "Usage: make migrate-to-version VERSION=<version_number>"
	@exit 1
endif
	@echo "Migrating to version $(VERSION)..."
	migrate -database "$(DB_URL)" -path migrations goto $(VERSION)

migrate-force-version:
ifeq ($(VERSION),)
	@echo "Usage: make migrate-force-version VERSION=<version_number>"
	@exit 1
endif
	@echo "Forcing version to $(VERSION)..."
	migrate -database "$(DB_URL)" -path migrations force $(VERSION)

# Target to print the current migration status
migrate-status:
	@echo "Checking migration status..."
	# This command might vary slightly depending on the migrate CLI version or if a direct status command is available
	# For now, we'll assume a tool like 'goose' or similar might have a status command.
	# 'migrate' itself doesn't have a direct status command, it shows status during 'up' or 'down'.
	# We can list files as a proxy or direct user to check schema_migrations table.
	@echo "Listing migration files in migrations/:"
	@ls -1 migrations
	@echo "Connect to DB and check 'schema_migrations' table for applied versions."

.PHONY: migrate-create migrate-up migrate-down-one migrate-to-version migrate-force-version migrate-status
