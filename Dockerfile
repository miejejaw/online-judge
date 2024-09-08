# Use the official Golang image as the base image
FROM golang:1.21-alpine

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and go sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Copy the .env file to the working directory
#COPY .env ./

# Set the working directory to the path where the main.go is located
WORKDIR /app/cmd/judge

# Build the Go judge
RUN go build -o /cmd/judge/main .

# Step 6: Use a lightweight image for running the app
#FROM alpine:latest

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["/cmd/judge/main"]