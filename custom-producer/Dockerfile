FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ansible-cred-producer ./main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /ansible-cred-producer /usr/local/bin/ansible-cred-producer
EXPOSE 8080
ENTRYPOINT ["ansible-cred-producer"]
