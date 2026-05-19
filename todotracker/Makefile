PORT ?= 8080

.PHONY: dev build run clean css css-watch test prod docker

# Start dev server with hot reload
dev:
	@echo "Starting dev server on :$(PORT)..."
	air

# Build Tailwind CSS (minified)
css:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/output.css --minify

# Watch Tailwind CSS for changes
css-watch:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/output.css --watch

# Build the binary (with Tailwind)
build: css
	go build -ldflags="-s -w" -o bin/todotracker .

# Run the built binary
run: build
	./bin/todotracker

# Production build: minified CSS + static Go binary
prod: css
	CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -o bin/todotracker .

# Build Docker image
docker:
	docker build -t todotracker:latest .

# Clean build artifacts
clean:
	rm -rf bin/ static/css/output.css

# Run tests
test:
	go test ./...
