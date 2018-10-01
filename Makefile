.PHONY: all compile clean test

all: clean test run

compile:
	go build -v -i -o bin/aleh cmd/aleh/main.go

run: compile
	./bin/aleh -c ./etc/config.json

clean:
	rm -rf bin

test:
	GOGC=off go test -v -race ./...