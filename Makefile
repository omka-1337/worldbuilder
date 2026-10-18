# `make` builds the UI and embeds it into a single binary.
# `make dev` runs the Go server and Vite (with hot reload) side by side,
# on the default library in Documents unless LIBRARY=path is given.

.PHONY: all web build dev test check release clean

all: build

web/node_modules: web/package.json
	cd web && npm install
	@touch $@

web: web/node_modules
	cd web && npm run build

build: web
	go build -o worldbuilder ./cmd/worldbuilder

dev: web/node_modules
	@trap 'kill 0' EXIT; \
	go run ./cmd/worldbuilder -no-open -no-tray $(if $(LIBRARY),-library "$(LIBRARY)") & \
	cd web && npm run dev

test:
	go test ./...

check: web/node_modules
	go vet ./...
	cd web && npm run check

# Cross-compiled binaries for distribution; the UI is embedded in each.
release: web
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags=-s -o dist/worldbuilder-linux-amd64       ./cmd/worldbuilder
	GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags=-s -o dist/worldbuilder-macos-arm64       ./cmd/worldbuilder
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=-s -o dist/worldbuilder-windows-amd64.exe ./cmd/worldbuilder

clean:
	rm -rf worldbuilder dist web/dist/assets web/dist/index.html
