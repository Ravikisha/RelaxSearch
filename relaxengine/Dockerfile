# Use the official Golang image as a base
FROM golang:1.21.5-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files, and download dependencies
COPY go.mod go.sum ./
RUN go mod tidy

# Copy the rest of the application code
COPY . .

# Build the Go application
RUN go build -o main ./cmd

# Run the Go application
CMD ["/app/main"]
