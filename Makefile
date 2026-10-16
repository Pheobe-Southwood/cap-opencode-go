# OpenCode Go provider plugin for CLIProxyAPI.
#
# Four shared libraries are produced from one source tree; each one declares a
# single upstream wire format so that CLIProxyAPI performs every client-protocol
# translation and the plugin only forwards bytes.

MODULE      := github.com/cpa-opencodego/cap-opencode-go
VERSION     ?= 1.0.0
GO          ?= go
DIST        := dist
GOFLAGS_EXTRA ?=
ROLES       := chat responses messages gemini
SUFFIX_chat      := cap-opencode-go
SUFFIX_responses := cap-opencode-go-responses
SUFFIX_messages  := cap-opencode-go-messages
SUFFIX_gemini    := cap-opencode-go-gemini

TARGETS := $(foreach role,$(ROLES),$(DIST)/$(SUFFIX_$(role))-v$(VERSION).so)

.PHONY: all build test vet fmt clean list install uninstall

all: build

build: $(TARGETS)

$(DIST):
	mkdir -p $(DIST)

$(DIST)/%-v$(VERSION).so: $(DIST)
	@role=$$(case "$*" in \
		cap-opencode-go) echo chat ;; \
		cap-opencode-go-responses) echo responses ;; \
		cap-opencode-go-messages) echo messages ;; \
		cap-opencode-go-gemini) echo gemini ;; \
		*) echo chat ;; esac); \
	echo "building $$role -> $@"; \
	CGO_ENABLED=1 $(GO) build -buildmode=c-shared -trimpath $(GOFLAGS_EXTRA) \
		-ldflags "-s -w -X main.role=$$role -X main.version=$(VERSION)" \
		-o $@ ./cmd/plugin; \
	rm -f $(basename $@).h

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(DIST)

list:
	@for role in $(ROLES); do echo "$$role -> $(DIST)/$(SUFFIX_$$role)-v$(VERSION).so"; done

# Install copies only the plugin libraries and prints the config block that has
# to be added to config.yaml. It never edits the host configuration.
install: build
	@bash scripts/install.sh "$(VERSION)"

uninstall:
	@bash scripts/uninstall.sh
