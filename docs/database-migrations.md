# Database Migrations

This document describes how database schema migrations are managed for the Crypto Trading Bot project using `golang-migrate/migrate`.

## Overview

We use `golang-migrate/migrate` to handle incremental and version-controlled changes to the database schema. This allows for easier collaboration, deployment, and rollback of database changes.

Migration files are located in the `migrations/` directory in the root of the repository. Each migration consists of an "up" SQL file (e.g., `000001_initial_schema.up.sql`) that applies changes, and a corresponding "down" SQL file (e.g., `000001_initial_schema.down.sql`) that reverts those changes.

The `golang-migrate/migrate` tool uses a special table in the database named `schema_migrations` to keep track of which migrations have been applied.

## Installation

Before you can run migration commands locally, you need to install the `golang-migrate/migrate` CLI tool.

**macOS (using Homebrew):**
```bash
brew install golang-migrate
```

**Using Go:**
Ensure you have Go installed and your `GOPATH/bin` (or `GOBIN`) is in your system's `PATH`.
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```
The `-tags 'postgres'` part is important as it includes the PostgreSQL driver.

## Makefile Targets

A `Makefile` is provided in the root of the repository to simplify common migration tasks. The database connection URL used by these commands can be configured by setting the `DB_URL` environment variable. It defaults to `postgres://user:password@localhost:5432/crypto_bot_db?sslmode=disable`.

**Available commands:**

*   **`make migrate-create NAME=<migration_name>`**
    *   Creates new up and down migration files in the `migrations/` directory with a sequential version number and the provided name.
    *   Example: `make migrate-create NAME=add_user_email_column`

*   **`make migrate-up`**
    *   Applies all pending "up" migrations to the database.
    *   Example: `DB_URL="postgres://myuser:mypass@myhost:5432/mydb?sslmode=disable" make migrate-up`

*   **`make migrate-down-one`**
    *   Reverts the last applied migration.

*   **`make migrate-to-version VERSION=<version_number>`**
    *   Migrates the database schema to a specific version. If the target version is higher than the current version, "up" migrations are applied. If lower, "down" migrations are applied.
    *   Example: `make migrate-to-version VERSION=000002`

*   **`make migrate-force-version VERSION=<version_number>`**
    *   Forcibly sets the database migration version to `<version_number>` in the `schema_migrations` table without applying or reverting any actual SQL. This is useful for correcting a dirty migration state but should be used with caution.
    *   Example: `make migrate-force-version VERSION=000001`

*   **`make migrate-status`**
    *   This target currently lists the migration files in the `migrations/` directory.
    *   To see the actual status of applied migrations, you need to connect to your database and inspect the `schema_migrations` table (e.g., `SELECT * FROM schema_migrations;`). The `migrate` CLI tool itself typically shows status during `up` or `down` operations.

## Schema Management

The `scripts/db/schema.sql` file, which previously represented the entire database schema, is now effectively replaced by the first migration file (`migrations/000001_initial_schema.up.sql`). All subsequent schema changes should be managed through new migration files created using `make migrate-create NAME=...`.

The `scripts/db/seed.sql` file can still be used for populating the database with initial or test data, but it should be run *after* migrations have been applied. Its management is separate from the schema migration process.
