APP_NAME=crypto-trading-bot-v4
AWS_REGION="eu-west-1"
REGISTRY_ID=$(aws ecr describe-registry --output text --query 'registryId' --region $AWS_REGION)
REGISTRY="${REGISTRY_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
REPO_PREFIX=${APP_NAME}
TAG=$(git rev-parse --short HEAD --)
HELM_CHART_DIR=helm
CHART_REPO=${APP_NAME}
CHART_TARGET_DIR="${HELM_CHART_DIR}/target"
CHART_VERSION="1.0.0-SHA${TAG}"

SERVICES=(pair-selector price-collector trading-engine)

set -e

HOME_DIR=$(pwd)

aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $REGISTRY
aws ecr get-login-password --region $AWS_REGION | helm registry login --username AWS --password-stdin $REGISTRY

# for service in "${SERVICES[@]}"; do
#     cd ${HOME_DIR}/services/${service}
    
#     go mod download

#     export CGO_ENABLED=0
#     export GOOS=linux
#     export GOARCH=arm64
#     go build -buildvcs=false -o app-arm64 ${HOME_DIR}/services/${service}/cmd

#     export GOARCH=amd64
#     go build -buildvcs=false -o app-amd64 ${HOME_DIR}/services/${service}/cmd

#     cd ${HOME_DIR}

#     aws ecr describe-repositories --region $AWS_REGION --repository-names ${REPO_PREFIX}/${service} || aws ecr create-repository --repository-name ${REPO_PREFIX}/${service} --region $AWS_REGION

#     docker buildx build --push --build-arg SERVICE_NAME=${service} --platform linux/amd64,linux/arm64 -t ${REGISTRY}/${REPO_PREFIX}/${service}:${TAG} .

#     rm -f ${HOME_DIR}/services/${service}/app-amd64 ${HOME_DIR}/services/${service}/app-arm64
# done

# aws ecr describe-repositories --region $AWS_REGION --repository-names ${CHART_REPO} || aws ecr create-repository --repository-name ${CHART_REPO} --region $AWS_REGION

helm package ${HELM_CHART_DIR} --destination ${CHART_TARGET_DIR} --version $CHART_VERSION
helm push ${CHART_TARGET_DIR}/${APP_NAME}-${CHART_VERSION}.tgz "oci://${REGISTRY}/"
rm -f ${CHART_TARGET_DIR}/*
