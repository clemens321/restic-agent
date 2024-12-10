# Build environment for restic-agent
FROM golang:1.23.4-alpine3.21 AS builder

# Install dependencies inside the build environment
RUN apk add --no-cache git

WORKDIR /app
COPY src .
ENV GO111MODULE=on
RUN go mod tidy && go build

# Productive environment
FROM alpine:3.21.0 AS runner

# from https://github.com/restic/restic/blob/master/docker/Dockerfile
RUN apk add --update --no-cache ca-certificates fuse openssh-client restic tzdata

# Add database clients
RUN apk add --no-cache postgresql-client mariadb-client

COPY --from=builder /app/restic-agent /usr/bin/

# Overwrite entrypoint /usr/bin/restic with our restic-agent application
ENTRYPOINT ["/usr/bin/restic-agent"]
