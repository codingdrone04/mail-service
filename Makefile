.PHONY: build run test check preview clean

build:
	go build -o bin/mail-service ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -rf bin/

test:
	go vet ./... && go test -race ./...

check:
	go run ./cmd/server -check

preview:
	PREVIEW_OUT=$(CURDIR)/preview.html go test ./internal/mail -run TestWritePreview -count=1
	@echo "open preview.html"
