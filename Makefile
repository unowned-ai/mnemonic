.PHONY: clean build rebuild test

build: bin/mnemonic

rebuild: clean build

bin/mnemonic:
	mkdir -p bin
	go mod tidy
	go build -o bin/mnemonic ./cmd/mnemonic

clean:
	rm -rf bin/*

test:
	go test ./... -v