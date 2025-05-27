-- Database Schema for Crypto Trading Bot
-- File: scripts/db/schema.sql

-- Extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Historical price data (minute-by-minute)
CREATE TABLE price_data (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    open DECIMAL(20,8) NOT NULL,
    high DECIMAL(20,8) NOT NULL,
    low DECIMAL(20,8) NOT NULL,
    close DECIMAL(20,8) NOT NULL,
    volume DECIMAL(20,8) NOT NULL,
    quote_volume DECIMAL(20,8) NOT NULL,
    change_rate DECIMAL(10,6),
    change_price DECIMAL(20,8),
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT unique_symbol_timestamp UNIQUE(symbol, timestamp)
);

-- Indexes for price_data
CREATE INDEX idx_price_data_symbol_timestamp ON price_data(symbol, timestamp DESC);
CREATE INDEX idx_price_data_timestamp ON price_data(timestamp DESC);
CREATE INDEX idx_price_data_symbol ON price_data(symbol);

-- Available trading pairs with metrics
CREATE TABLE trading_pairs (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL UNIQUE,
    base_asset VARCHAR(10) NOT NULL,
    quote_asset VARCHAR(10) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    daily_volume DECIMAL(20,8),
    daily_volume_usdt DECIMAL(20,8),
    volatility_score DECIMAL(10,6),
    atr_14 DECIMAL(20,8),
    correlation_btc DECIMAL(5,4),
    price_change_24h DECIMAL(10,6),
    last_price DECIMAL(20,8),
    last_updated TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Index for trading_pairs
CREATE INDEX idx_trading_pairs_volume ON trading_pairs(daily_volume_usdt DESC);
CREATE INDEX idx_trading_pairs_volatility ON trading_pairs(volatility_score DESC);

-- Active pairs selected for trading
CREATE TABLE selected_pairs (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    selection_score DECIMAL(10,6) NOT NULL,
    volatility_24h DECIMAL(10,6),
    volume_24h_usdt DECIMAL(20,8),
    atr_score DECIMAL(10,6),
    volume_score DECIMAL(10,6),
    correlation_score DECIMAL(10,6),
    risk_level VARCHAR(10) DEFAULT 'medium',
    status VARCHAR(20) DEFAULT 'active',
    selected_at TIMESTAMP DEFAULT NOW(),
    last_evaluated TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_selected_pairs_symbol FOREIGN KEY (symbol) REFERENCES trading_pairs(symbol)
);

-- Index for selected_pairs
CREATE INDEX idx_selected_pairs_score ON selected_pairs(selection_score DESC);
CREATE INDEX idx_selected_pairs_status ON selected_pairs(status);

-- Trading configurations per pair
CREATE TABLE trading_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pair_id BIGINT NOT NULL,
    strategy_type VARCHAR(20) DEFAULT 'grid',
    grid_levels INTEGER DEFAULT 10,
    price_range_min DECIMAL(20,8),
    price_range_max DECIMAL(20,8),
    position_size_usdt DECIMAL(20,8) DEFAULT 100.00,
    stop_loss_percent DECIMAL(5,4) DEFAULT 0.05,
    take_profit_percent DECIMAL(5,4) DEFAULT 0.03,
    max_positions INTEGER DEFAULT 5,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_trading_configs_pair FOREIGN KEY (pair_id) REFERENCES selected_pairs(id)
);

-- Trading positions and orders
CREATE TABLE positions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pair_id BIGINT NOT NULL,
    config_id UUID NOT NULL,
    side VARCHAR(10) NOT NULL, -- 'buy' or 'sell'
    quantity DECIMAL(20,8) NOT NULL,
    entry_price DECIMAL(20,8) NOT NULL,
    current_price DECIMAL(20,8),
    unrealized_pnl DECIMAL(20,8) DEFAULT 0,
    realized_pnl DECIMAL(20,8) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'open', -- 'open', 'closed', 'partial'
    order_id VARCHAR(50), -- KuCoin order ID
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    closed_at TIMESTAMP,
    CONSTRAINT fk_positions_pair FOREIGN KEY (pair_id) REFERENCES selected_pairs(id),
    CONSTRAINT fk_positions_config FOREIGN KEY (config_id) REFERENCES trading_configs(id)
);

-- Index for positions
CREATE INDEX idx_positions_pair_status ON positions(pair_id, status);
CREATE INDEX idx_positions_created_at ON positions(created_at DESC);

-- Orders history
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    position_id UUID,
    pair_id BIGINT NOT NULL,
    kucoin_order_id VARCHAR(50) UNIQUE,
    side VARCHAR(10) NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'market', 'limit'
    quantity DECIMAL(20,8) NOT NULL,
    price DECIMAL(20,8),
    filled_quantity DECIMAL(20,8) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    fee DECIMAL(20,8) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    filled_at TIMESTAMP,
    CONSTRAINT fk_orders_position FOREIGN KEY (position_id) REFERENCES positions(id),
    CONSTRAINT fk_orders_pair FOREIGN KEY (pair_id) REFERENCES selected_pairs(id)
);

-- Index for orders
CREATE INDEX idx_orders_kucoin_id ON orders(kucoin_order_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- System configuration
CREATE TABLE system_config (
    id SERIAL PRIMARY KEY,
    config_key VARCHAR(50) NOT NULL UNIQUE,
    config_value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert default system configurations
INSERT INTO system_config (config_key, config_value, description) VALUES
('max_active_pairs', '8', 'Maximum number of pairs to trade simultaneously'),
('min_volume_threshold_usdt', '1000000', 'Minimum daily volume in USDT for pair selection'),
('volatility_min_threshold', '0.03', 'Minimum volatility threshold (3%)'),
('volatility_max_threshold', '0.08', 'Maximum volatility threshold (8%)'),
('pair_evaluation_interval_hours', '4', 'Hours between pair selection evaluations'),
('price_collection_interval_seconds', '60', 'Seconds between price data collection');

-- Views for analytics
CREATE VIEW v_active_trading_summary AS
SELECT 
    sp.symbol,
    sp.selection_score,
    sp.volume_24h_usdt,
    sp.volatility_24h,
    tp.last_price,
    tp.price_change_24h,
    COUNT(p.id) as open_positions,
    COALESCE(SUM(p.unrealized_pnl), 0) as total_unrealized_pnl
FROM selected_pairs sp
JOIN trading_pairs tp ON sp.symbol = tp.symbol
LEFT JOIN positions p ON sp.id = p.pair_id AND p.status = 'open'
WHERE sp.status = 'active'
GROUP BY sp.id, sp.symbol, sp.selection_score, sp.volume_24h_usdt, sp.volatility_24h, tp.last_price, tp.price_change_24h
ORDER BY sp.selection_score DESC;

CREATE VIEW v_price_data_latest AS
SELECT DISTINCT ON (symbol) 
    symbol, 
    timestamp, 
    open, 
    high, 
    low, 
    close, 
    volume, 
    quote_volume,
    change_rate
FROM price_data 
ORDER BY symbol, timestamp DESC;