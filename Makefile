export GOMODULES11 = on

all: dep tests

dep:
	go mod tidy
	go mod vendor

tests:
	go test -v ./...

clean:
	go clean
	rm -fr vendor

.PHONY: all dep clean install
