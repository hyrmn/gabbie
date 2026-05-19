PORT ?= 8080

.PHONY: dev build run clean css css-watch test

# Start dev server with hot reload
dev:
	@echo "Starting dev server on :$(PORT)..."
	air

# Build Tailwind CSS
css:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/output.css --minify

# Watch Tailwind CSS for changes
css-watch:
	npx @tailwindcss/cli -i static/css/input.css -o static/css/output.css --watch

# Build the binary
build: css
	go build -ldflags="-s -w" -o bin/todotracker .

# Run the built binary
run: build
	./bin/todotracker

# Clean build artifacts
clean:
	rm -rf bin/ static/css/output.css

# Run tests
test:
	go test ./...
