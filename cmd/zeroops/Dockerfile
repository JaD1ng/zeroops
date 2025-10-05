FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/zeroops ./cmd/zeroops

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/zeroops /app/zeroops
# 复制配置文件目录
COPY configs/ /app/configs/
EXPOSE 8080
ENTRYPOINT ["/app/zeroops"]
CMD ["-f", "/app/config.json"]


