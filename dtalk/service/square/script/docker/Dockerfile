FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/square ./cmd/square

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/square /app/square
COPY sql/schema.sql /app/sql/schema.sql
COPY web/admin /app/web/admin
ENV SQUARE_SCHEMA_PATH=/app/sql/schema.sql
ENV SQUARE_ADMIN_WEB=/app/web/admin
ENV TZ=Asia/Shanghai
EXPOSE 18100
CMD ["/app/square"]
