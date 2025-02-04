# Build stage
FROM golang:1.23-alpine AS go-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o mqtt-iomeesage .

# Final image
FROM alpine:latest

COPY --from=go-builder /app/mqtt-iomeesage /bin/mqtt-iomeesage

ENTRYPOINT ["/bin/mqtt-iomeesage"]
