# === Import variables from kucoincreds file ===
NAMESPACE=crypto-trading
APP_NAME=crypto-trading-bot-v4
AWS_REGION=eu-west-1
TAG=$(git rev-parse --short HEAD --)
CHART="oci://929572853995.dkr.ecr.${AWS_REGION}.amazonaws.com/${APP_NAME}"
CHART_VERSION="1.0.0-SHA${TAG}"
REGISTRY_ID=$(aws ecr describe-registry --output text --query 'registryId' --region $AWS_REGION)
REGISTRY="${REGISTRY_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

export $(grep -v '^#' kucoincreds | xargs)

aws ecr get-login-password --region $AWS_REGION | helm registry login --username AWS --password-stdin $REGISTRY
helm -n ${NAMESPACE} upgrade \
    --history-max 3 \
    --set "pairSelector.image.tag=${TAG}" \
    --set "priceCollector.image.tag=${TAG}" \
    --set "tradingEngine.image.tag=${TAG}" \
    --set "markets.kucoin.apiKey=${KUCOIN_API_KEY}" \
    --set "markets.kucoin.apiSecret=${KUCOIN_API_SECRET}" \
    --set "markets.kucoin.apiPassphrase=${KUCOIN_API_PASSPHRASE}" \
    --install $APP_NAME $CHART --version $CHART_VERSION