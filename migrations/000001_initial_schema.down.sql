-- Down migration for 000001_initial_schema.up.sql
-- File: migrations/000001_initial_schema.down.sql

-- Drop views first to avoid dependency issues
DROP VIEW IF EXISTS v_price_data_latest;
DROP VIEW IF EXISTS v_active_trading_summary;

-- Drop tables in reverse order of creation, considering foreign key constraints
-- Note: If foreign key constraints have ON DELETE CASCADE, the order might be less strict for those.
-- However, explicit reverse order is safer.

DROP TABLE IF EXISTS system_config; -- No incoming FKs

DROP TABLE IF EXISTS orders; -- FK to positions, selected_pairs
DROP TABLE IF EXISTS positions; -- FK to trading_configs, selected_pairs
DROP TABLE IF EXISTS trading_configs; -- FK to selected_pairs
DROP TABLE IF EXISTS selected_pairs; -- FK to trading_pairs
DROP TABLE IF EXISTS trading_pairs; -- No incoming FKs from other tables being dropped now (price_data is separate)
DROP TABLE IF EXISTS price_data; -- No incoming FKs

-- Drop extensions last
DROP EXTENSION IF EXISTS "uuid-ossp";
