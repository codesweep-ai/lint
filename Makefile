# cs-lint — build/test/release.
# `make build` produces bin/cs-lint via goreleaser (single host target,
# version-stamped, CGO_ENABLED=0). Falls back to plain `go build` if goreleaser
# is absent. See .goreleaser.yaml.

GORELEASER ?= goreleaser
BIN        := bin/cs-lint
PKG        := ./cmd/cs-lint
PREFIX     ?= $(HOME)/.local
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X github.com/codesweep-ai/lint/internal/cli.Version=$(VERSION)
# Tracked files where git knows them, every .go file where it does not — a
# fresh clone before its first commit has nothing tracked, and an empty list
# makes `gofmt -l` read stdin and hang rather than check anything.
GO_FILES   := $(shell git ls-files '*.go' 2>/dev/null | grep . || find . -name '*.go' -not -path './bin/*' -not -path './dist/*')

# Coverage is measured on every `make test` rather than in a mode of its own,
# because a number nobody measures is a number that only falls.
#
# The suite writes Go binary coverage data into $(COVERDIR) and merges it with
# `go tool covdata`, rather than writing a text profile with -coverprofile. With
# -coverpkg=./... every package's test binary emits a block set for every other
# package, and merging those text profiles counts each block once per binary:
# one package measured at 93% reads as 48% in the merged file. The binary format
# merges by union, which is the number that means something.
#
# -test.gocoverdir must be absolute: `go test` runs each package's test binary
# with that package's directory as its working directory, so a relative path
# would scatter the data one directory per package.
COVERDIR   ?= .coverage
COVER_ABS  := $(abspath $(COVERDIR))
COVERPKG    = $(shell go list ./... | paste -sd, -)
COVERFLAGS  = -covermode=atomic -coverpkg=$(COVERPKG)
# The floor the suite has to clear. Raised when a tier lands, never lowered to
# make a run green.
COVER_MIN  ?= 70

.PHONY: help build build-go install uninstall test test-race coverage coverage-check ci \
        vet fmt fmt-check check lint deadcode prose refs oss surface self \
        snapshot release release-check clean

.DEFAULT_GOAL := help

## help: list available targets (this menu)
help:
	@echo "cs-lint make targets:"
	@grep -E '^## [a-z][a-z0-9-]*: ' $(MAKEFILE_LIST) | sed -E 's/^## ([^:]+): (.*)/  \1|\2/' | column -t -s '|'
	@echo ""
	@echo "  PREFIX=$(PREFIX) (install location; override with make install PREFIX=/usr/local)"

## build: host binary at bin/cs-lint via goreleaser (single target)
build:
	@mkdir -p $(dir $(BIN))
	@if command -v $(GORELEASER) >/dev/null 2>&1; then \
		VERSION='$(VERSION)' $(GORELEASER) build --single-target --snapshot --clean --output $(BIN); \
	else \
		echo "goreleaser not found; using go build (run 'make build-go' explicitly to force)"; \
		$(MAKE) build-go; \
	fi

## build-go: host binary at bin/cs-lint via plain go build
build-go:
	@mkdir -p $(dir $(BIN))
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

## versions: what this build is made of — this repo's binary, every pinned tool,
## the Go toolchain, and whether a workspace is overriding the go.mod pins. Each
## line is read by asking that binary its own version. It deliberately depends on
## nothing and runs from source: reporting a version must not trigger a build.
.PHONY: versions
versions:
	@if out="$$(go run -ldflags '$(LDFLAGS)' $(PKG) version 2>&1)"; then \
		printf '%-12s %-38s %s\n' '$(notdir $(BIN))' "$$(printf '%s\n' "$$out" | awk 'NR==1{print $$2}')" 'this repo'; \
	else \
		printf '%-12s %s\n' '$(notdir $(BIN))' "FAILED — $$(printf '%s\n' "$$out" | head -1)"; \
	fi
	@for t in $$(go list tool 2>/dev/null); do \
		if out="$$(go tool $$t version 2>&1)"; then \
			printf '%-12s %s\n' "$$(basename $$t)" "$$(printf '%s\n' "$$out" | awk 'NR==1{print $$2}')"; \
		else \
			printf '%-12s %s\n' "$$(basename $$t)" "FAILED — $$(printf '%s\n' "$$out" | head -1)"; \
		fi; \
	done
	@printf '%-12s %s\n' 'go' "$$(go env GOVERSION)"
	@w="$$(go env GOWORK)"; \
	case "$$w" in \
		''|off) printf '%-12s %s\n' 'workspace' 'off — versions above are go.mod pins' ;; \
		*)      printf '%-12s %s\n' 'workspace' "$$w — local checkouts override the go.mod pins" ;; \
	esac

## repin: move every codesweep-ai tool pin to its branch tip, then report. Uses
## GOPROXY=direct because the module proxy caches branch resolution and `@main`
## can come back a commit behind origin/main. Uses GOWORK=off so this edits the
## recorded pins even while a workspace is serving local checkouts.
.PHONY: repin
repin:
	@tools="$$(go list tool 2>/dev/null | grep codesweep-ai || true)"; \
	if [ -z "$$tools" ]; then \
		echo "no codesweep-ai tools declared yet — add the first with:" >&2; \
		echo "  GOPROXY=direct go get -tool github.com/codesweep-ai/lint/cmd/cs-lint@main" >&2; \
		exit 1; \
	fi; \
	GOWORK=off GOPROXY=direct go get -tool $$(echo "$$tools" | sed 's|$$|@main|')
	@GOWORK=off go mod tidy
	@$(MAKE) versions

## install: build and copy the binary into $(PREFIX)/bin
install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/cs-lint
	@echo "installed $(PREFIX)/bin/cs-lint"

## uninstall: remove the binary from $(PREFIX)/bin
uninstall:
	rm -f $(PREFIX)/bin/cs-lint

## test: the unit suite, with coverage into $(COVERDIR)
test:
	@rm -rf $(COVER_ABS) && mkdir -p $(COVER_ABS)
	go test $(COVERFLAGS) ./... -args -test.gocoverdir=$(COVER_ABS)

## test-race: the unit suite under the race detector
test-race:
	go test -race ./...

## coverage: the per-function coverage of the last run
coverage:
	@go tool covdata textfmt -i=$(COVER_ABS) -o=$(COVER_ABS)/merged.out
	@go tool cover -func=$(COVER_ABS)/merged.out

## coverage-check: fail when total coverage is under $(COVER_MIN)%
coverage-check:
	@go tool covdata textfmt -i=$(COVER_ABS) -o=$(COVER_ABS)/merged.out
	@total=$$(go tool cover -func=$(COVER_ABS)/merged.out | awk '/^total:/ {gsub(/%/,"",$$NF); print $$NF}'); \
	echo "coverage: $$total% (floor $(COVER_MIN)%)"; \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { exit (t+0 >= m+0) ? 0 : 1 }' \
		|| { echo "coverage is under the floor" >&2; exit 1; }

## vet: the compiler-adjacent checks
vet:
	go vet ./...

## fmt: format every tracked Go file
fmt:
	gofmt -w $(GO_FILES)

## fmt-check: fail when a tracked Go file is not formatted
fmt-check:
	@out=$$(gofmt -l $(GO_FILES)); \
	if [ -n "$$out" ]; then echo "not gofmt'd:"; echo "$$out"; exit 1; fi

## lint: the Go rules from .golangci.yml (see that file for what is on and why)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed; see https://golangci-lint.run/welcome/install/" >&2; \
		exit 1; \
	}
	golangci-lint run

## deadcode: functions no entry point reaches, which `unused` cannot see
deadcode:
	@command -v deadcode >/dev/null 2>&1 || { \
		echo "deadcode is not installed; go install golang.org/x/tools/cmd/deadcode@latest" >&2; \
		exit 1; \
	}
	@out=$$(deadcode -test ./...); \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

## prose: check how this repository's documents are written
prose: build
	$(BIN) prose

## refs: check that everything the documents point at is there
refs: build
	$(BIN) refs

## oss: check that this repository can be published
oss: build
	$(BIN) oss

## surface: check the docs against the binary, the code and the build
surface: build
	$(BIN) surface

## self: run every linter over this repository
self: prose refs oss surface

## check: the one command before pushing
check: fmt-check vet lint deadcode test coverage-check prose refs oss surface

# say prints a heading above each gate, so a long run reads as a list rather
# than as a wall. Bold where a terminal is reading it and plain where a pipe
# is: `make ci > ci.log` should leave a log somebody can read. The escapes are
# the same ones scripts/check.sh uses in tracer, which is where the shape came
# from.
define say
@if [ -t 1 ]; then printf '\n\033[1m==> %s\033[0m\n' "$(1)"; else printf '\n==> %s\n' "$(1)"; fi
endef

## ci: every gate the CI workflow runs, on this machine
##
## One Linux leg of .github/workflows/ci.yml, in the order CI runs it, so a
## red build is something you can see before you push rather than after. What
## it cannot reproduce it names on the way out: a run that skipped a gate must
## never read as a run that ran them all.
ci:
	$(call say,the gate a contributor runs before pushing)
	@$(MAKE) --no-print-directory check
	$(call say,build)
	@$(MAKE) --no-print-directory build
	$(call say,release manifest)
	@$(MAKE) --no-print-directory release-check
	@printf '\nci: every gate ran. Not reproduced here: build-test on macOS.\n'

## snapshot: build every release target without publishing
snapshot:
	VERSION='$(VERSION)' $(GORELEASER) release --snapshot --clean --skip=publish,sign

## release-check: validate the release manifest
release-check:
	$(GORELEASER) check

## release: cut a release from the current tag
release:
	$(GORELEASER) release --clean

## clean: remove build output and coverage data
clean:
	rm -rf bin dist $(COVERDIR)
