export GOTOOLCHAIN := go1.27.3
export MACOSX_DEPLOYMENT_TARGET := 13.0
ifeq ($(shell uname -s),Darwin)
export CGO_CFLAGS := $(CGO_CFLAGS) -mmacosx-version-min=13.0
export CGO_LDFLAGS := $(CGO_LDFLAGS) -mmacosx-version-min=13.0
endif

.PHONY: test test-thumbnail-helper test-thumbnail-helper-race build-thumbnail-helper test-go test-go-race test-python vet build smoke test-flutter format

format:
	cd backend && gofmt -w ./cmd ./internal

test: test-runtime test-go test-go-race test-python vet test-thumbnail-helper test-thumbnail-helper-race

test-go:
	cd backend && go test ./...

test-go-race:
	cd backend && go test -race ./...

test-python:
	python3 -m unittest discover -s scripts/tests -v

vet:
	cd backend && go vet ./...

build:
	mkdir -p build
	cd backend && go build -trimpath -o ../build/telegram-media-downloader-backend ./cmd/manager

smoke: build-thumbnail-helper
	./scripts/smoke.sh
	python3 scripts/library_smoke.py
	python3 scripts/thumbnail_smoke.py

test-flutter:
	cd apps/desktop && flutter pub get && flutter analyze && flutter test

test-thumbnail-helper:
	cd thumbnail-helper && go test ./... && go vet ./...

test-thumbnail-helper-race:
	cd thumbnail-helper && go test -race ./...

build-thumbnail-helper:
	mkdir -p build
	cd thumbnail-helper && go build -trimpath -ldflags="-w" -o ../build/tdl-tmm-previews-v3 .

.PHONY: test-runtime build-runtime security public-source
test-runtime:
	cd runtime/tdl && go test ./... && go vet ./...

build-runtime:
	mkdir -p build
	cd runtime/tdl && go build -trimpath -ldflags="-w -X github.com/iyear/tdl/pkg/consts.Version=0.20.4 -X github.com/iyear/tdl/pkg/consts.Commit=security-gotd-0.145.1" -o ../../build/tdl-tmm github.com/iyear/tdl

security:
	python3 scripts/security_checks.py

public-source:
	mkdir -p build
	python3 scripts/public_source.py --zip build/telegram-media-downloader-source.zip
