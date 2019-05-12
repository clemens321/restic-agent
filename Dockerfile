# Build environment for restic
FROM golang:1.12.1-alpine AS builder-restic

WORKDIR /go/src/github.com/restic/restic
COPY restic .
RUN ["go", "run", "build.go"]

# Build environment for restic-agent
FROM golang:1.12.1-alpine AS builder

# Install dependencies inside the build environment
RUN apk add --no-cache git

WORKDIR /app
COPY src .
ENV GO111MODULE=on
RUN go mod tidy && go build

# Productive environment
FROM restic/restic:0.9.5 AS runner

# Add database clients
RUN apk add --no-cache postgresql-client mariadb-client

# Optional: use self-compiled version from git submodule
COPY --from=builder-restic /go/src/github.com/restic/restic/restic /usr/bin/
COPY --from=builder /app/restic-agent /usr/bin/

# Overwrite entrypoint /usr/bin/restic with our restic-agent application
ENTRYPOINT ["/usr/bin/restic-agent"]
