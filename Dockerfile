## Builder image

FROM golang:1.22-alpine as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o dist/davd .

## Runtime Image

FROM alpine:latest
COPY --from=builder /app/dist/davd /usr/bin/davd
ENV DAVD_ADDR=0.0.0.0
ENV DAVD_PORT=8080
ENV DAVD_ROOT_DIR=/var/data/davd/default_root
CMD [ "davd", "server", "run" ]