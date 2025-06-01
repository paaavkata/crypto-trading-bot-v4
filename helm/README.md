# Crypto Trading Bot Helm Chart

This Helm chart deploys the Crypto Trading Bot application, which consists of the following services:
- Price Collector
- Pair Selector
- Trading Engine

## Prerequisites

- Kubernetes 1.19+
- Helm 3+
- A running PostgreSQL database accessible from the Kubernetes cluster. The connection URI for this database must be stored in a Kubernetes secret.

## Database Secret

Before deploying, create a secret containing the database URI:

```bash
kubectl create secret generic crypto-trading-pguser-postgres \
  --from-literal=uri='postgresql://USER:PASSWORD@HOST:PORT/DATABASE_NAME' \
  --namespace <your-namespace>
```

Replace `USER`, `PASSWORD`, `HOST`, `PORT`, `DATABASE_NAME`, and `<your-namespace>` with your actual database credentials and the target namespace for deployment. The name of this secret (`crypto-trading-pguser-postgres`) and the key (`uri`) are referenced in `values.yaml` under `secretEnvVars`.

## Kucoin API Credentials

This chart requires a Kubernetes secret to store your Kucoin API credentials. Create the secret before deploying the chart:

```bash
kubectl create secret generic kucoin-api-credentials \
  --from-literal=api-key='YOUR_API_KEY' \
  --from-literal=api-secret='YOUR_API_SECRET' \
  --from-literal=api-passphrase='YOUR_API_PASSPHRASE' \
  --namespace <your-namespace>
```
Replace `YOUR_API_KEY`, `YOUR_API_SECRET`, `YOUR_API_PASSPHRASE`, and `<your-namespace>` with your actual Kucoin API credentials and the namespace where the chart will be deployed.
The secret name (default: `kucoin-api-credentials`) can be overridden in `values.yaml` via the `kucoinApiSecretName` field.

## Installation

To install the chart with the release name `crypto-trading-bot`:

```bash
helm install crypto-trading-bot . --namespace <your-namespace>
```

You can override values from `values.yaml` using the `--set` flag or by providing a custom values file:

```bash
helm install crypto-trading-bot . --namespace <your-namespace> -f my-values.yaml
```

## Configuration

The following table lists the configurable parameters of the Crypto Trading Bot chart and their default values.

| Parameter                       | Description                                                        | Default                                       |
| ------------------------------- | ------------------------------------------------------------------ | --------------------------------------------- |
| `registry`                      | Docker registry for the service images                             | `929572853995.dkr.ecr.eu-west-1.amazonaws.com` |
| `app`                           | Application name, used as part of the image path                   | `crypto-trading-bot-v4`                       |
| `kucoinApiSecretName`           | Name of the K8s secret for Kucoin API credentials                  | `kucoin-api-credentials`                      |
| `envVars[].name`                | Global environment variable name                                   | `LOG_LEVEL`                                   |
| `envVars[].value`               | Global environment variable value                                  | `info`                                        |
| `secretEnvVars[].name`          | Name of environment variable to be sourced from a secret           | `DB_URI`                                      |
| `secretEnvVars[].secretName`    | Name of the K8s secret for the environment variable                | `crypto-trading-pguser-postgres`              |
| `secretEnvVars[].secretKey`     | Key within the K8s secret                                          | `uri`                                         |
| `priceCollector.image.name`     | Image name for Price Collector service                             | `price-collector`                             |
| `priceCollector.image.tag`      | Image tag for Price Collector service                              | `latest`                                      |
| `priceCollector.image.pullPolicy`| Image pull policy for Price Collector                              | `Always`                                      |
| `priceCollector.deployment.replicas` | Number of replicas for Price Collector                           | `1`                                           |
| `priceCollector.envVars[]`      | Environment variables specific to Price Collector                  | (see `values.yaml`)                           |
| `pairSelector.image.name`       | Image name for Pair Selector service                               | `pair-selector`                               |
| `pairSelector.image.tag`        | Image tag for Pair Selector service                                | `latest`                                      |
| `pairSelector.image.pullPolicy` | Image pull policy for Pair Selector                                | `Always`                                      |
| `pairSelector.deployment.replicas` | Number of replicas for Pair Selector                             | `1`                                           |
| `pairSelector.envVars[]`        | Environment variables specific to Pair Selector                    | (see `values.yaml`)                           |
| `tradingEngine.image.name`      | Image name for Trading Engine service                              | `trading-engine`                              |
| `tradingEngine.image.tag`       | Image tag for Trading Engine service                               | `latest`                                      |
| `tradingEngine.image.pullPolicy`| Image pull policy for Trading Engine                               | `Always`                                      |
| `tradingEngine.deployment.replicas` | Number of replicas for Trading Engine                            | `1`                                           |
| `tradingEngine.envVars[]`       | Environment variables specific to Trading Engine                   | (see `values.yaml`)                           |

---

*This README provides basic guidance. For detailed configuration, refer to the `values.yaml` file.*
