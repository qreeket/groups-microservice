FROM golang:alpine AS builder
WORKDIR /
COPY . .
RUN go mod download
EXPOSE 9905
RUN go build -o groups main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /groups .
COPY --from=builder /.env ./
EXPOSE 9905
CMD ["./groups"]
