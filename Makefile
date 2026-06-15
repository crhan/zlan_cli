GO ?= go
GORELEASER ?= go run github.com/goreleaser/goreleaser/v2@v2.16.0
GOCACHE ?= /tmp/zlan-cli-go-cache
GOMODCACHE ?= /tmp/zlan-cli-modcache

export GOCACHE
export GOMODCACHE

.PHONY: test vet ci build release-check release-snapshot release-tag clean

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

ci: test vet release-check

build:
	$(GO) build -trimpath -o dist/zlan .

release-check:
	@if git remote get-url origin >/dev/null 2>&1; then \
		$(GORELEASER) check; \
	else \
		echo "No origin remote configured; running local snapshot validation instead."; \
		$(GORELEASER) release --snapshot --clean --skip=archive; \
	fi

release-snapshot:
	$(GORELEASER) release --snapshot --clean

release-tag:
	@test -n "$(VERSION)" || (echo "usage: make release-tag VERSION=v0.1.0" >&2; exit 2)
	@case "$(VERSION)" in v*) ;; *) echo "VERSION must start with v, got $(VERSION)" >&2; exit 2;; esac
	git tag -a "$(VERSION)" -m "Release $(VERSION)"

clean:
	rm -rf dist/
