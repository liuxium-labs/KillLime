GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

.PHONY: lint lint-fix build release

lint:
	$(GOLANGCI_LINT) run ./...
	cd deps/proxy && $(GOLANGCI_LINT) run ./...

lint-fix:
	$(GOLANGCI_LINT) run --fix ./...
	cd deps/proxy && $(GOLANGCI_LINT) run --fix ./...

build:
	mkdir -p release
	cd example/default && go build -trimpath -ldflags "-s -w" -o ../../release/killlime-proxy.exe .
	cd example/dragonfly && go build -trimpath -ldflags "-s -w" -o ../../release/killlime-dragonfly.exe .

release: build
	cd release && powershell -Command "Compress-Archive -Path 'killlime-proxy.exe','killlime-dragonfly.exe' -DestinationPath 'killlime-v0.2.2-windows-x64.zip' -Force"
