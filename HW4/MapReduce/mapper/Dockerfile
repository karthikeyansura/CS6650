# Syntax=docker/dockerfile:1

FROM golang:1.25 AS build-stage

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the binary.
# CGO_ENABLED=0 ensures a static binary (no dependencies on host OS libraries)
RUN CGO_ENABLED=0 GOOS=linux go build -o /mapper-service

# Switch to a minimal Alpine runtime image
FROM alpine:edge AS build-release-stage

# Install CA certificates required for HTTPS requests
RUN apk add --no-cache ca-certificates

WORKDIR /

# Copy only the binary from the build stage
COPY --from=build-stage /mapper-service /mapper-service

# Document that the container listens on port 8080
EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["/mapper-service"]