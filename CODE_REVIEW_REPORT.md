## Code Review Report: Crypto Trading Bot

**Date:** October 26, 2023
**Version:** Initial Review

---

### Overall Assessment

In its current state, the solution **would NOT work reliably or safely in a live production environment.** While many foundational components are in place and the microservice architecture is sensible, there are several major roadblocks:

1.  **Placeholder Trading Logic:** The core trading signal generation in the `trading-engine` is currently random. This is the most significant blocker to any actual trading functionality.
2.  **Missing Critical Trading Features:** Essential features like stop-loss/take-profit execution within the basic strategy, and robust order status tracking (handling fills, partial fills, rejections from the exchange) are absent. Without these, real funds would be at high risk.
3.  **Security of API Credentials:** Kucoin API keys are intended to be passed via Helm `values.yaml`, which is insecure. While currently empty, this setup encourages unsafe practices.
4.  **Production Readiness of Deployments:** The Helm charts lack crucial elements for production like proper secret management for API keys, specific image tagging, liveness/readiness probes, and configurable resource limits.
5.  **Undefined Migration Tooling:** While schema files exist, the process for applying and managing database migrations is unclear, risking schema inconsistencies.

The system demonstrates a good separation of concerns and has a decent database schema. However, the gaps in trading logic, security, and operational readiness are too substantial for live deployment.

---

### Service-Specific Feedback

#### `price-collector`
*   **Key Findings:**
    *   Collects ticker and symbol data from Kucoin, respecting rate limits.
    *   Handles API and parsing errors for individual tickers resiliently.
    *   Stores data in the `price_data` table with an ON CONFLICT clause for updates.
    *   Timestamp for collected data is truncated to the minute of fetch time, not necessarily the Kucoin event time for each ticker.
    *   `parseFloatSafe` returns 0.0 for empty strings which might mask missing data.
*   **Recommendations:**
    *   **Timestamp Accuracy:** Prioritize using timestamps provided by the Kucoin API for each ticker, if available, instead of the batch fetch time. This is crucial for data accuracy.
    *   **Empty String Handling:** Re-evaluate returning `0.0` from `parseFloatSafe` for empty strings. Consider returning an error or `nil` if an empty string means "data not available" rather than a literal zero, to be handled by a distinct normalization/cleaning step.
    *   **Configuration:** Make rate limit parameters (25 req/sec) and retention days for `CleanupOldData` configurable through environment variables or `system_config` table.

#### `pair-selector`
*   **Key Findings:**
    *   Analyzes trading pairs based on volume, volatility, and correlation (hardcoded against "BTC-USDT").
    *   Filters pairs based on configurable criteria (min/max volume, volatility).
    *   Scores pairs and determines a risk level ("low", "medium", "high").
    *   Updates `trading_pairs` metrics and stores selected pairs in `selected_pairs`.
    *   `UpdateTradingPairMetrics` saves raw volatility as `volatility_score` in the DB, which might be a naming mismatch.
    *   `fmt.Sprintf` is used for injecting `hours` into a SQL query in `GetPriceHistory`, a minor SQLi risk if `hours` weren't an integer.
*   **Recommendations:**
    *   **Configurable Correlation Benchmark:** Make the correlation benchmark symbol (currently "BTC-USDT") configurable.
    *   **Configurable Risk Thresholds:** Risk level determination thresholds should be configurable (e.g., via `system_config` or environment variables) rather than hardcoded.
    *   **Metric Naming:** Clarify/fix the potential mismatch between `analysis.Volatility` and `volatility_score` when calling `UpdateTradingPairMetrics`.
    *   **SQL Parameterization:** Use parameterized queries for all variable inputs in SQL, even for integers like `hours` in `GetPriceHistory`, as a best practice.
    *   **Delisting Strategy:** Implement a strategy for handling delisted pairs (e.g., marking them inactive if they disappear from the exchange's symbol list).

#### `trading-engine`
*   **Key Findings:**
    *   **Signal Generation is Placeholder:** Current signal logic is random and unsuitable for trading.
    *   **Basic Strategy Limitations:** The `executeBasicStrategy` is long-only, exits only profitable longs on a "SELL" signal, and lacks stop-loss/take-profit execution.
    *   **Order Status Tracking Absent:** Creates "pending" orders in DB but no visible mechanism to update their status from Kucoin (fills, rejections).
    *   Relies on `RiskManager` (details not shown) for pre-trade checks.
    *   Grid strategy logic is mentioned but not provided. Default grid config uses volatility for range but `PriceRangeMin/Max` are initially 0 and marked as "set dynamically".
    *   Kucoin exchange interaction uses hardcoded "limit" orders and fixed precision for quantity/price.
*   **Recommendations:**
    *   **Implement Real Signal Logic:** This is the highest priority. Replace random signals with well-defined trading strategies.
    *   **Implement Stop-Loss/Take-Profit:** Add logic to `executeBasicStrategy` (or a new default strategy) to automatically close positions when SL/TP levels (from `TradingConfig`) are hit.
    *   **Develop Order Status Tracking:** Implement a robust system to poll Kucoin for order status updates and reflect these in the `orders` and `positions` tables. This is critical for accurate position management.
    *   **Exchange Trading Rules:** Fetch and use Kucoin's symbol-specific trading rules (min/max order size, price/quantity precision) to prevent order rejections.
    *   **Strategy Implementation:** Fully implement the `GridStrategy`, including dynamic price range setting, or replace it with other defined strategies.
    *   **Order Type Flexibility:** Allow strategies to specify order types (e.g., market, limit) if needed.

---

### Shared Components Feedback

#### Kucoin API Client (`shared/pkg/kucoin/client.go`)
*   **Key Findings:**
    *   Correctly implements Kucoin V2 authentication.
    *   Provides methods for fetching tickers, symbols, and placing orders.
    *   Uses `resty/v2` for HTTP calls, including retries.
    *   Response handling is verbose (multiple marshal/unmarshal steps).
    *   Lacks context propagation for requests.
*   **Recommendations:**
    *   **Context Propagation:** Add `context.Context` to all API call methods (e.g., `GetAllTickers(ctx context.Context, ...)`) and pass it to `resty` requests.
    *   **Efficient Response Handling:** Refactor response parsing to be more efficient, e.g., by using `json.RawMessage` for the `Data` field or `resty`'s `SetResult`/`SetError` capabilities.
    *   **Rate Limiting Awareness:** While services implement their own, consider adding optional, configurable client-side rate limiting as a utility if many services were to use this client without their own safeguards.
    *   **Expand Endpoint Coverage:** Gradually add more Kucoin API endpoints as needed by the services (e.g., get balances, cancel orders, get order status, kline data).

#### PostgreSQL Utility (`shared/pkg/database/postgres.go`)
*   **Key Findings:**
    *   Provides robust connection pooling and a health check mechanism.
    *   Uses `github.com/lib/pq` driver.
*   **Recommendations:**
    *   **Configurable Pool Settings:** Make connection pool parameters (max open, max idle, etc.) configurable via the `database.Config` struct instead of being hardcoded.
    *   **Transaction Helper (Optional):** Consider adding a utility function to simplify transaction management (begin, commit/rollback) for repository methods.

---

### Database Schema & Migrations
*   **Key Findings:**
    *   Schema is generally well-structured, covering price data, pairs, selection, configurations, positions, and orders.
    *   Uses `DECIMAL` for financial data and UUIDs for some primary keys.
    *   Appropriate indexes are defined for common query patterns.
    *   A migration file `001_initial_schema.sql` exists and is identical to `scripts/db/schema.sql`, suggesting an intent for a migration system.
    *   **No actual migration tooling or `schema_migrations` table is evident.**
*   **Recommendations:**
    *   **Implement Migration Tooling:** Adopt a standard database migration tool (e.g., `golang-migrate/migrate`, `Pressly/goose`, `Flyway`). This tool should manage a `schema_migrations` table and apply migrations automatically.
    *   **Schema Version Control:** Ensure `scripts/db/schema.sql` is either generated from migrations or serves as the definitive initial schema from which migrations evolve. Avoid manual, uncoordinated edits.
    *   **Data Integrity:** Consider using PostgreSQL `ENUM` types or `CHECK` constraints for `status` and `type` fields to enforce valid values.
    *   **Foreign Key Policies:** Review and explicitly define `ON DELETE` policies (e.g., `CASCADE`, `SET NULL`, `RESTRICT`) for foreign keys based on desired data lifecycle and referential integrity.
    *   **Down Migrations:** Implement "down" migrations for each schema change to allow for rollbacks.

---

### Helm Deployment (Kubernetes)
*   **Key Findings:**
    *   Provides basic deployment templates for each service.
    *   `DB_URI` is correctly sourced from a Kubernetes secret.
    *   Kucoin API keys are sourced from `values.yaml` (currently empty, but an insecure practice).
    *   Resource requests/limits are hardcoded and identical across services.
    *   Uses `latest` image tags.
    *   Lacks liveness/readiness probes and HPAs.
*   **Recommendations:**
    *   **Secure API Key Management:** **CRITICAL:** Store Kucoin API keys in Kubernetes secrets and mount them via `valueFrom.secretKeyRef`, similar to `DB_URI`. Remove them from `values.yaml`.
    *   **Configurable Resources:** Define resource requests/limits in `values.yaml` per service, not hardcoded in templates.
    *   **Use Specific Image Tags:** Replace `latest` tags with immutable tags (e.g., Git SHAs, semantic versions) in `values.yaml` for reliable deployments and rollbacks.
    *   **Implement Probes:** Add liveness and readiness probes to all service deployments to improve stability and enable smoother updates.
    *   **Consider HPAs:** For services with variable load, implement HorizontalPodAutoscalers.
    *   **NetworkPolicies:** Define NetworkPolicies to restrict inter-pod communication based on the principle of least privilege.
    *   **Helm Helper Templates:** For common sections (like environment variable setup), consider using Helm helper templates to reduce redundancy.
    *   **Document Prerequisites:** Clearly document the need for the `crypto-trading-pguser-postgres` secret to exist before deployment.

---

### General Recommendations for Efficiency & Profitability

#### Algorithm/Strategy Improvements
*   **Beyond Basic:** Move beyond simple MA crossovers (implied by random signal reasons) or basic grid. Explore strategies like:
    *   Mean-reversion strategies with statistical significance tests (e.g., ADF for stationarity, Johansen for cointegration if trading pairs of assets).
    *   Momentum/trend-following strategies (e.g., Donchian channels, breakout systems).
    *   Volume-Profile analysis.
    *   Order Book Imbalance strategies.
*   **Parameter Optimization:** Implement robust backtesting and use techniques like walk-forward optimization to find optimal strategy parameters. Avoid overfitting.
*   **Portfolio Construction:** If trading multiple pairs, consider portfolio effects (correlation, diversification) rather than treating each pair in isolation for capital allocation.
*   **Machine Learning (Advanced):** Explore ML models for signal generation or regime identification, but be mindful of overfitting and computational costs.

#### Risk Management Enhancements
*   **Dynamic Position Sizing:** Instead of fixed USDT position size, consider dynamic sizing based on:
    *   Volatility (e.g., smaller size for higher volatility).
    *   Account equity (e.g., fixed fractional).
    *   Strategy conviction/signal strength.
*   **Portfolio-Level Risk:** Implement overall portfolio risk limits (e.g., max drawdown, max concurrent open risk).
*   **Slippage & Execution Costs:** Factor in estimated slippage and trading fees into PnL calculations and strategy profitability assessments. Market orders (if used) are more prone to slippage.
*   **Circuit Breakers:** Implement global or per-pair circuit breakers to halt trading during extreme market events or if the bot behaves erratically.

#### Observability (Logging, Metrics, Tracing)
*   **Structured Logging:** Ensure all services use structured logging (e.g., JSON with `logrus`) consistently. Include correlation IDs for tracing requests across services.
*   **Key Metrics:**
    *   **Trading Engine:** PnL (realized/unrealized), trade volume, win/loss ratio, Sharpe ratio, drawdown, order fill rates, slippage.
    *   **Pair Selector:** Number of pairs analyzed/selected, score distributions.
    *   **Price Collector:** Data collection latency, error rates from exchange, number of records stored.
    *   **System:** API latencies, DB query times, message queue lengths (if applicable).
*   **Monitoring & Alerting:** Use Prometheus for metrics collection and Grafana for dashboards. Set up alerts (e.g., via Alertmanager) for critical errors, high drawdown, system failures.
*   **Distributed Tracing:** Consider implementing distributed tracing (e.g., OpenTelemetry, Jaeger/Zipkin) to understand request flows and pinpoint bottlenecks in inter-service communication.

#### Security Best Practices
*   **Secrets Management:** Reinforce: API keys, database credentials, and any other secrets must be stored securely (e.g., Kubernetes Secrets, HashiCorp Vault) and injected into applications, not hardcoded or in version control.
*   **Principle of Least Privilege:**
    *   Database users should have only the necessary permissions on tables/schemas.
    *   API keys should have restricted permissions (e.g., trading enabled, withdrawals disabled).
*   **Network Segmentation:** Use Kubernetes NetworkPolicies to restrict traffic between pods and to/from external services.
*   **Regular Dependency Scanning:** Scan application dependencies for known vulnerabilities.
*   **Secure Endpoints:** If any service exposes an API (even internal), ensure it's secured (e.g., mTLS, token-based auth).

#### Testing and CI/CD
*   **Unit Tests:** Expand unit test coverage for all services, especially complex logic in signal generation, strategy execution, and financial calculations.
*   **Integration Tests:**
    *   Test interactions between services (e.g., `pair-selector` output being consumed by `trading-engine`).
    *   Test interactions with database and mock exchange APIs.
*   **Backtesting Framework:** Develop or integrate a robust backtesting framework to evaluate trading strategies against historical data. This is crucial for strategy validation.
*   **Paper Trading:** Before live trading, run the bot in a paper trading mode (simulated trading with live market data) for an extended period to validate performance and identify bugs. The `KUCOIN_SANDBOX: "true"` setting is a good start.
*   **CI/CD Pipeline:** Implement a CI/CD pipeline (e.g., Jenkins, GitLab CI, GitHub Actions) to automate testing, building Docker images, and deploying to Kubernetes environments.

---

### Conclusion

The project has a good architectural foundation with its microservices approach and initial schema. However, it is far from being a functional or safe trading bot.

**Most Critical Issues:**

1.  **Non-Existent Trading Strategy:** The `trading-engine`'s signal generation is random.
2.  **Lack of Basic Risk Controls in Trading:** No automated stop-loss/take-profit in the basic strategy.
3.  **Insecure API Key Handling:** Kucoin keys in Helm values.
4.  **Missing Order Status Management:** The bot doesn't track if orders are actually filled.
5.  **Immature Deployment Configuration:** `latest` tags, no probes, hardcoded resources.

**Recommended Next Steps:**

1.  **Security First:** Immediately refactor Helm charts to use Kubernetes secrets for Kucoin API keys.
2.  **Core Trading Logic:**
    *   Implement a first, simple, but complete trading strategy in `trading-engine`.
    *   Add stop-loss and take-profit execution to this strategy.
    *   Develop a mechanism to track order statuses from Kucoin and update the internal state.
3.  **Productionize Deployments:**
    *   Use specific image tags.
    *   Add liveness/readiness probes.
    *   Make resource limits configurable in `values.yaml`.
4.  **Database Migrations:** Implement a proper migration tool and workflow.
5.  **Testing:** Focus on building out a backtesting framework and paper trading capabilities to validate strategies before any consideration of live trading.

Addressing these foundational issues is essential before moving on to more advanced strategy development or performance optimization. The project has potential but requires significant development in core trading functionality and operational practices.
