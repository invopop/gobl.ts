.PHONY: generate build test typecheck format lint clean

# Regenerate the TypeScript types from the GOBL version pinned in go.mod.
# To generate against a local GOBL checkout, uncomment the replace directive in
# go.mod and pass the version explicitly, e.g.
#   make generate VERSION=v0.506.0-dev
VERSION ?=
generate:
	go run ./cmd/generate $(if $(VERSION),-version $(VERSION),)

# Re-vendor the example envelopes used by the corpus type test. Needs a local
# checkout of github.com/invopop/gobl, since the examples are not embedded in
# the Go module.
.PHONY: fixtures
GOBL_PATH ?= ../gobl
fixtures:
	node scripts/vendor-fixtures.mjs $(GOBL_PATH)

build:
	npm run build

test:
	npm test

# Runs the tests that call the public GOBL API.
.PHONY: test-live
test-live:
	GOBL_LIVE=1 npx vitest run test/live.test.ts

typecheck:
	npx tsc --noEmit

format:
	npx prettier --write .

lint:
	gofmt -l cmd/
	go vet ./...
	npx prettier --check .

clean:
	rm -rf dist node_modules
