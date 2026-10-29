BINARY_NAME ?= genshin-cleaner-backend.exe

.PHONY: lint lint-fix test build clean security ci release dry-release

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

lint-fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --fix

test:
	go test $(if $(filter 1,$(shell go env CGO_ENABLED)),-race) ./...

build: export GOOS = windows
build: export GOARCH = amd64
build: export CGO_ENABLED = 0
build:
	go tool rsrc -arch amd64 -manifest backend.manifest -o rsrc_windows_amd64.syso
	go build -mod=readonly -trimpath -ldflags "-s -w" -o "$(BINARY_NAME)" .

clean:
	rm -f "$(BINARY_NAME)" rsrc_windows_*.syso

security:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

ci: lint test security build

release:
	goreleaser release --clean

dry-release:
	goreleaser release --snapshot --clean
