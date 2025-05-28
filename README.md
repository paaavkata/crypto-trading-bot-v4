# Crypto Trading Bot - Three Microservices Architecture

This project implements a comprehensive cryptocurrency trading bot using three microservices:

## Architecture Overview

### 1. Price Collector Service (`price-collector`)
- **Purpose**: Collects real-time price data from KuCoin API
- **Functionality**: 
  - Fetches all trading pairs every minute via REST API
  - Stores historical price data in PostgreSQL
  - Updates trading pairs metadata
  - Cleanup old data (30-day retention)
- **Port**: 8080 (health checks)

### 2. Pair Selector Service (`pair-selector`) 
- **Purpose**: Analyzes and selects optimal trading pairs
- **Functionality**:
  - Implements comprehensive pair selection framework
  - Analyzes volatility (3-8% target range)
  - Filters by volume (>$1M USDT daily)
  - Calculates correlation with BTC
  - Selects top 8 pairs for active trading from 20-pair watchlist
  - Runs evaluation every 4-6 hours
- **Port**: 8081 (health checks)

### 3. Trading Engine Service (`trading-engine`)
- **Purpose**: Executes trading strategies on selected pairs
- **Functionality**:
  - Grid trading strategy implementation
  - Risk management (stop-loss, take-profit)
  - Position management
  - Order execution via KuCoin API
  - Real-time signal generation
- **Port**: 8082 (health checks)

## Key Features

### Pair Selection Criteria
- **Volume Threshold**: Minimum $1M USDT daily volume
- **Volatility Range**: 3-8% daily price fluctuations
- **Risk Distribution**: Balanced selection across low/medium/high risk pairs
- **Dynamic Scoring**: Weighted scoring system combining volume, volatility, ATR, and correlation metrics

### Trading Strategy
- **Grid Trading**: Automated buy/sell orders at predetermined price levels
- **Risk Management**: 5% stop-loss, 3% take-profit defaults
- **Position Limits**: Maximum 5 positions per pair
- **Diversification**: Trades up to 8 pairs simultaneously

### Technical Implementation
- **Language**: Go 1.23
- **Database**: PostgreSQL with optimized schemas
- **Exchange**: KuCoin REST API integration
- **Deployment**: Kubernetes-ready with health checks
- **Monitoring**: Structured logging and metrics

## Database Schema

The database schema is managed via `golang-migrate/migrate`. The initial schema is defined in `migrations/000001_initial_schema.up.sql`.
All subsequent schema changes are applied through new migration files.

For detailed instructions on managing database migrations, installing the `migrate` CLI, and using the provided `Makefile` targets, please refer to [docs/database-migrations.md](./docs/database-migrations.md).

### Core Tables (as defined in initial schema)
- `price_data`: Minute-by-minute OHLCV data
- `trading_pairs`: Available pairs with metrics
- `selected_pairs`: Currently selected pairs for trading
- `trading_configs`: Strategy configurations per pair
- `positions`: Open/closed trading positions
- `orders`: Order history and status
- `system_config`: System-wide configuration key-value store

## Environment Variables

### Common
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- `KUCOIN_API_KEY`, `KUCOIN_API_SECRET`, `KUCOIN_PASSPHRASE`
- `LOG_LEVEL` (debug, info, warn, error)

### Service-Specific
- **Price Collector**: `COLLECTION_INTERVAL_SECONDS`, `BATCH_SIZE`
- **Pair Selector**: `EVALUATION_INTERVAL_HOURS`, `MIN_VOLUME_USDT`, `MAX_ACTIVE_PAIRS`
- **Trading Engine**: `TRADING_INTERVAL_SECONDS`, `DEFAULT_POSITION_SIZE_USDT`

## Deployment

Each service is designed as an independent microservice with:
- Docker containerization support
- Kubernetes health checks (`/health`, `/ready`)
- Graceful shutdown handling
- Database connection pooling
- Rate limiting for API calls

## Getting Started

1. Set up PostgreSQL database
2. Apply database migrations using the `Makefile` (e.g., `make migrate-up`). See [docs/database-migrations.md](./docs/database-migrations.md) for details.
3. Configure environment variables (especially `DB_URL` for migrations if not using the default).
4. Deploy services to Kubernetes.
5. Monitor logs and health endpoints.

The system is designed to be highly resilient with proper error handling, retry logic, and monitoring capabilities suitable for production cryptocurrency trading operations.