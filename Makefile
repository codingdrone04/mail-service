.PHONY: build run clean

build:
	go build -o bin/mail-service ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -rf bin/
