FROM arm32v7/golang:alpine AS builder

WORKDIR /src
COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG TARGETARM

RUN gofmt -s -w ./
RUN go get
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -v -o pair



FROM hypriot/rpi-alpine-scratch
WORKDIR /
COPY --from=builder /src/pair .
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/pair"]