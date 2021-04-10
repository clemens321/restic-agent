# Build environment for restic
FROM golang:1.16.3-alpine AS builder-restic

WORKDIR /go/src/github.com/restic/restic
COPY restic .
RUN ["go", "run", "build.go"]

# Build environment for restic-agent
FROM golang:1.16.3-alpine AS builder

# Install dependencies inside the build environment
RUN apk add --no-cache git

WORKDIR /app
COPY src .
ENV GO111MODULE=on
RUN go mod tidy && go build

# Productive environment
FROM alpine:3.13 AS runner

# from https://github.com/restic/restic/blob/master/docker/Dockerfile
RUN apk add --update --no-cache ca-certificates fuse openssh-client tzdata

# Add database clients
RUN apk add --no-cache postgresql-client mariadb-client

# Optional: use self-compiled version from git submodule
COPY --from=builder-restic /go/src/github.com/restic/restic/restic /usr/bin/
COPY --from=builder /app/restic-agent /usr/bin/

# Overwrite entrypoint /usr/bin/restic with our restic-agent application
ENTRYPOINT ["/usr/bin/restic-agent"]
