# Step 1: Build Stage
FROM golang:1.24-alpine AS builder

# Install git and ca-certificates
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# Create an unprivileged user
ENV USER=appuser
ENV UID=10001
RUN adduser \
  --disabled-password \
  --gecos "" \
  --home "/nonexistent" \
  --shell "/sbin/nologin" \
  --no-create-home \
  --uid "${UID}" \
  "${USER}"

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify

# Copy the rest of the source code
COPY . .

# Build the binary statically from main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go-backend main.go

# Step 2: Final Stage (Scratch - completely empty image)
FROM scratch

# Import the user, group, and CA certificates
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Import the compiled binary
COPY --from=builder /go-backend /go-backend

# Import the required directories for the frontend
COPY --from=builder /app/static /static
COPY --from=builder /app/templates /templates

# Enforce the unprivileged user
USER appuser:appuser

EXPOSE 8080

ENTRYPOINT ["/go-backend"]
