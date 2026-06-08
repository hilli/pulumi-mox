PROVIDER  := mox
# Single source of truth is provider/provider.go; override with `make VERSION=x.y.z`.
VERSION   ?= $(shell sed -n 's/^var Version = "\(.*\)"/\1/p' provider/provider.go)
PLUGIN    := pulumi-resource-$(PROVIDER)
BIN       := bin/$(PLUGIN)
PKG       := github.com/hilli/pulumi-mox
LDFLAGS   := -X $(PKG)/provider.Version=$(VERSION)

.PHONY: build install_plugin gen_sdk gen_go_sdk sdk_build tidy test vet clean e2e

# Build the plugin binary into bin/.
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./provider/cmd/$(PLUGIN)

# Install the freshly-built plugin into the local Pulumi plugin cache so
# examples/ can resolve it without a registry.
install_plugin: build
	pulumi plugin install resource $(PROVIDER) $(VERSION) --file $(BIN) --reinstall

# Generate every language SDK from the built plugin's schema into sdk/.
# The Go SDK (sdk/go) is committed; all other languages stay gitignored and are
# meant for ad-hoc inspection only — consumers generate them via `pulumi package add`.
gen_sdk: build
	pulumi package gen-sdk ./$(BIN)

# Regenerate ONLY the committed Go SDK at the current VERSION, into sdk/go.
# Run this whenever the schema changes, then commit the result.
gen_go_sdk: build
	pulumi package gen-sdk ./$(BIN) --language go --version $(VERSION) -o sdk

# Tidy and compile the committed Go SDK module (separate go.mod under sdk/go).
sdk_build:
	cd sdk/go && go mod tidy && go build ./...

# Live end-to-end test against a local `mox localserve`. Starts an ephemeral
# mox (or reuses one with MOX_E2E_REUSE=1), runs the examples/yaml stack, then
# tears everything down. See examples/yaml/e2e.sh for env knobs.
e2e: install_plugin
	./examples/yaml/e2e.sh

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test ./...

# Remove build output. The committed Go SDK (sdk/go) is intentionally NOT removed;
# other generated language SDKs under sdk/ are gitignored (use `git clean -fdx sdk`).
clean:
	rm -rf bin dist
