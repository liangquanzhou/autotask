unexport GOROOT

.PHONY: build install test scan doctor diff

build:
	go build ./...

install:
	go build -o $(HOME)/.local/bin/autotask .

test:
	go test ./...

scan:
	go run . scan --personal

doctor:
	go run . doctor --personal

diff:
	go run . diff
