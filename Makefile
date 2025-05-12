.PHONY: clean build rebuild test

build: bin/recall

rebuild: clean build

bin/recall:
	mkdir -p bin
	go mod tidy
	go build -o bin/recall ./cmd/recall

clean:
	rm -rf bin/*

test:
	go test ./... -v