# Go Memory Stress Test Application
# Compatible with Azure App Service (Linux) and Azure Container Apps

# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /app

# Copy go mod file
COPY go.mod ./

# Download dependencies (if any)
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o memory-stress .

# Runtime stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12 AS runtime

WORKDIR /app

# Copy the binary from build stage
COPY --from=build /app/memory-stress .

# Set environment variables
ENV PORT=8080

# Expose port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/memory-stress"]
