# relaxweb/Dockerfile
FROM golang:1.21.5-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the code into the container
COPY . .

# Download Go dependencies
RUN go mod tidy

# Expose port 7000 for the search API
EXPOSE 7000

# Run the main application
CMD ["go", "run", "main.go"]
