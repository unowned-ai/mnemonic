.PHONY: clean build rebuild test format

build: format bin/mnemonic

rebuild: clean build

bin/mnemonic:
	mkdir -p bin
	go mod tidy
	go build -o bin/mnemonic ./cmd/mnemonic

format:
	gofmt -w .

clean:
	rm -rf bin/*

test:
	go test ./... -v