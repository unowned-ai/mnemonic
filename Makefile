.PHONY: clean build rebuild test format

build: format bin/recall

rebuild: clean build

bin/recall:
	mkdir -p bin
	go mod tidy
	go build -o bin/recall ./cmd/recall

format:
	gofmt -w .

clean:
	rm -rf bin/*

test:
	go test ./... -v