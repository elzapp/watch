PREFIX ?= $(HOME)/.local

.PHONY: build install clean

build:
	go build -o gowatch .

install: build
	install -d $(PREFIX)/bin
	install -m 755 gowatch $(PREFIX)/bin/watch

clean:
	rm -f gowatch
