# Builder
FROM whatwewant/builder-go:v1.25-1 as builder

WORKDIR /build

COPY go.mod ./

COPY go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
  go build \
  -trimpath \
  -ldflags '-w -s -buildid=' \
  -v -o migrate ./cmd/migrate

# Server
FROM whatwewant/alpine:v3-1

LABEL MAINTAINER="Zero<tobewhatwewant@gmail.com>"

LABEL org.opencontainers.image.source="https://github.com/go-idp/migrate"

COPY --from=builder /build/migrate /bin

RUN migrate --version

# CMD migrate server
