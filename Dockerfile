FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/cd-agent-linux main.go

FROM alpine:latest
RUN apk add --no-cache bash ca-certificates
WORKDIR /app
COPY --from=builder /app/cd-agent-linux ./cd-agent-linux

ENTRYPOINT ["./cd-agent-linux"]
