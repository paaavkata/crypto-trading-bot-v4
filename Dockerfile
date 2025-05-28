FROM gcr.io/distroless/base-debian10

ARG TARGETPLATFORM
ARG SERVICE_NAME

COPY services/${SERVICE_NAME}/app-${TARGETPLATFORM##*/} /app

EXPOSE 8080
CMD ["/app"]
