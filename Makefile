BIN := bin/linear-orch
PKG := ./cmd/linear-orch

.PHONY: build run test fmt vet clean install

build:
	go build -ldflags="-linkmode=external" -o $(BIN) $(PKG)
	@if [ "$$(uname)" = "Darwin" ]; then codesign --force --sign - $(BIN); fi

run: build
	$(BIN) $(ARGS)

vet:
	go vet ./...

fmt:
	go fmt ./...

test:
	go test ./...

clean:
	rm -rf bin

install: build
	install -m 0755 $(BIN) $${HOME}/.local/bin/linear-orch
