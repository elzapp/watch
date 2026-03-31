INSTALL_DIR := $(firstword $(foreach d,$(HOME)/.bin $(HOME)/.local/bin,$(if $(findstring :$d:,:$(PATH):),$d)))

.PHONY: build install clean

build:
	go build -o gowatch .

install: build
ifeq ($(INSTALL_DIR),)
	$(error Neither ~/.bin nor ~/.local/bin found in $$PATH)
endif
	install -d $(INSTALL_DIR)
	install -m 755 gowatch $(INSTALL_DIR)/watch

clean:
	rm -f gowatch
