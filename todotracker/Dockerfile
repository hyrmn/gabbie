# Multi-stage build for To-Do Tracker
# Stage 1: Build Tailwind CSS + Go binary
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache nodejs npm

WORKDIR /app

# Copy Go module files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy Tailwind deps
COPY package.json package-lock.json* ./
RUN npm install

# Copy source
COPY . .

# Build Tailwind CSS (minified)
RUN npx @tailwindcss/cli -i static/css/input.css -o static/css/output.css --minify

# Build Go binary (static, no CGO)
RUN CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -o /todotracker .

# Stage 2: Minimal runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /todotracker .

# Create directory for SQLite database
RUN mkdir -p /data

ENV DB_PATH=/data/todotracker.db
ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["/todotracker"]
