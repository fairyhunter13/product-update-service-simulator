FROM golang:1.25.1 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/product-update-service-simulator ./cmd/product-update-service-simulator

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/product-update-service-simulator /product-update-service-simulator
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/product-update-service-simulator"]
