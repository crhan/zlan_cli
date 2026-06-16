GO ?= go
GORELEASER ?= go run github.com/goreleaser/goreleaser/v2@v2.16.0
GOCACHE ?= /tmp/zlan-cli-go-cache
GOMODCACHE ?= /tmp/zlan-cli-modcache

export GOCACHE
export GOMODCACHE

.PHONY: test vet ci build install release-check release-snapshot release-tag clean

# 本地安装目录。默认 ~/.local/bin —— 它在 PATH 里;Go 默认的 ~/go/bin 在本机不在 PATH,
# 直接 `go install` 会装到那儿但命令调不到。可用 `make install INSTALL_DIR=...` 覆盖。
INSTALL_DIR ?= $(HOME)/.local/bin
# 版本/commit/date 注入,与 GoReleaser(.goreleaser.yaml)同一套变量。
# 版本号取最近的 tag(strip v 前缀,与 release 二进制一致,不拖 -N-g/-dirty 后缀);
# 工作树是否脏由 commit 行的 -dirty 标记如实反映,版本号本身保持干净。
INSTALL_VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//')
INSTALL_COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)$(shell git diff --quiet 2>/dev/null || echo -dirty)
INSTALL_DATE    ?= $(shell date -u +%FT%TZ)
INSTALL_LDFLAGS := -s -w \
	-X zlan/internal/cli.versionOverride=$(INSTALL_VERSION) \
	-X zlan/internal/cli.commit=$(INSTALL_COMMIT) \
	-X zlan/internal/cli.date=$(INSTALL_DATE)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

ci: test vet release-check

build:
	$(GO) build -trimpath -o dist/zlan .

install:
	GOBIN=$(INSTALL_DIR) CGO_ENABLED=0 $(GO) install -trimpath -ldflags "$(INSTALL_LDFLAGS)" .
	@echo "installed zlan $(INSTALL_VERSION) -> $(INSTALL_DIR)/zlan"

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
