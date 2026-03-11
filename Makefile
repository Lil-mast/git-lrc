.PHONY: build build-all build-local build-local-test run run-fake-review bump release clean test testall test-pkg upload-secrets download-secrets

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
BINARY_NAME=lrc
GH_REPO=HexmosTech/git-lrc
GH=/usr/bin/gh
ENV_VARS=B2_KEY_ID B2_APP_KEY B2_BUCKET_NAME B2_BUCKET_ID

# Build lrc for the current platform
build:
	$(GOBUILD) -o $(BINARY_NAME) .

# Build lrc for all platforms (linux/darwin/windows × amd64/arm64)
# Output: dist/<platform>/lrc[.exe] + SHA256SUMS
# Version is extracted from appVersion constant in main.go
build-all:
	@echo "🔨 Building lrc CLI for all platforms..."
	@python scripts/lrc_build.py -v build

# Build lrc locally for the current platform and install
build-local:
	@echo "🔨 Building lrc CLI locally (dirty tree allowed)..."
	@go build -o /tmp/lrc .
	@mkdir -p $(HOME)/.local/bin
	@install -m 0755 /tmp/lrc $(HOME)/.local/bin/lrc
	@cp $(HOME)/.local/bin/lrc $(HOME)/.local/bin/git-lrc
	@echo "✅ Installed lrc and git-lrc to ~/.local/bin"
	@case ":$$PATH:" in *:$(HOME)/.local/bin:*) ;; *) echo "⚠️  ~/.local/bin is not in PATH. Run: source ~/.lrc/env" ;; esac

# Build lrc locally in fake-review mode for E2E testing (no AI calls)
build-local-test:
	@echo "🔨 Building lrc CLI locally in FAKE REVIEW mode..."
	@go build -ldflags "-X main.reviewMode=fake" -o /tmp/lrc .
	@mkdir -p $(HOME)/.local/bin
	@install -m 0755 /tmp/lrc $(HOME)/.local/bin/lrc
	@cp $(HOME)/.local/bin/lrc $(HOME)/.local/bin/git-lrc
	@echo "✅ Installed fake-review lrc and git-lrc to ~/.local/bin"
	@echo "   Use WAIT=30s make run-fake-review (or set LRC_FAKE_REVIEW_WAIT)"
	@case ":$$PATH:" in *:$(HOME)/.local/bin:*) ;; *) echo "⚠️  ~/.local/bin is not in PATH. Run: source ~/.lrc/env" ;; esac

# Run the locally built lrc CLI (pass args via ARGS="--flag value")
run: build-local
	@echo "▶️ Running lrc CLI locally..."
	@lrc $(ARGS)

# Run fake review flow using fake-review build (defaults: WAIT=30s, TMP_REPO=/tmp/lrc-fake-review-repo)
run-fake-review: build-local-test
	@WAIT=$${WAIT:-30s} TMP_REPO=$${TMP_REPO:-/tmp/lrc-fake-review-repo} scripts/fake_review.sh $(ARGS)

# Bump lrc version by editing appVersion in main.go
# Prompts for version bump type (patch/minor/major)
bump:
	@echo "📝 Bumping lrc version..."
	@python scripts/lrc_build.py bump

# Build and upload lrc to Backblaze B2
release:
	@echo "🚀 Building and releasing lrc..."
	@python scripts/lrc_build.py -v release

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf dist/ $(BINARY_NAME)
	@echo "✅ Clean complete"

# Run tests
test:
	$(GOTEST) -count=1 ./...

# Run all tests (alias for test)
testall: test

# Run tests for a specific package (example: make test-pkg PKG=./internal/naming)
test-pkg:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-pkg PKG=./path/to/package"; \
		exit 1; \
	fi
	$(GOTEST) -count=1 $(PKG)

# Upload .env variables to GitHub repo variables
upload-secrets:
	@if [ ! -f .env ]; then echo "Error: .env file not found"; exit 1; fi
	@echo "Uploading .env to GitHub variables for $(GH_REPO)..."
	@$(GH) variable set -f .env --repo $(GH_REPO)
	@echo "✅ Uploaded. Current GitHub variables:"
	@$(GH) variable list --repo $(GH_REPO)

# Download GitHub repo variables to .env
download-secrets:
	@if [ -f .env ]; then \
		echo "⚠️  .env already exists (modified: $$(stat -c '%y' .env 2>/dev/null || stat -f '%Sm' .env 2>/dev/null))"; \
		printf "Overwrite? [y/N]: "; \
		read ans; \
		if [ "$$ans" != "y" ] && [ "$$ans" != "Y" ]; then \
			echo "Aborted."; \
			exit 1; \
		fi; \
	fi
	@echo "Downloading GitHub variables for $(GH_REPO) to .env..."
	@rm -f .env.tmp
	@for var in $(ENV_VARS); do \
		val=$$($(GH) variable get $$var --repo $(GH_REPO) 2>/dev/null); \
		if [ $$? -eq 0 ]; then \
			echo "$$var=$$val" >> .env.tmp; \
		else \
			echo "⚠️  Variable $$var not found on GitHub"; \
		fi; \
	done
	@mv .env.tmp .env
	@echo "✅ Downloaded to .env"
