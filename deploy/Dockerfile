FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -o /out/indexer ./cmd/indexer && \
    CGO_ENABLED=0 go build -trimpath -o /out/replay ./cmd/replay

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D explorer
COPY --from=build /out/ /app/
COPY tests/fixtures/testnet.json /fixtures/testnet.json
USER explorer
EXPOSE 8080
CMD ["/app/api"]
