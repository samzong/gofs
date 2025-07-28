# Homebrew
# 使用立即展开避免重复计算，正确处理 git describe 输出格式
CLEAN_VERSION := $(shell echo $(VERSION) | sed 's/^v//' | cut -d'-' -f1)
HOMEBREW_TAP_REPO := homebrew-tap
HOMEBREW_TAP_OWNER := samzong
FORMULA_FILE := Formula/$(PROJECT_NAME).rb
BRANCH_NAME := update-$(PROJECT_NAME)-$(CLEAN_VERSION)

# 添加默认值支持
DRY_RUN ?= 0
VERBOSE ?= 0

SUPPORTED_ARCHS := Darwin_x86_64 Darwin_arm64 Linux_x86_64 Linux_arm64

# 调试支持
ifeq ($(VERBOSE),1)
    CURL_VERBOSE := -v
    ECHO_PREFIX := [VERBOSE]
else
    CURL_VERBOSE := -s
    ECHO_PREFIX := 
endif

##@ Homebrew
.PHONY: update-homebrew
update-homebrew: ## Update Homebrew formula with new version
	@echo "==> Starting Homebrew formula update process..."
	@if [ -z "$(GH_PAT)" ]; then \
		echo "❌ Error: GH_PAT environment variable is required"; \
		exit 1; \
	fi

	@echo "==> Current version information:"
	@echo "    - VERSION: $(VERSION)"
	@echo "    - CLEAN_VERSION: $(CLEAN_VERSION)"

	@echo "==> Preparing working directory..."
	@rm -rf tmp && mkdir -p tmp
	
	@echo "==> Cloning Homebrew tap repository..."
	@cd tmp && git clone https://$(GH_PAT)@github.com/samzong/$(HOMEBREW_TAP_REPO).git
	@cd tmp/$(HOMEBREW_TAP_REPO) && echo "    - Creating new branch: $(BRANCH_NAME)" && git checkout -b $(BRANCH_NAME)

	@echo "==> Processing architectures and calculating checksums..."
	@cd tmp/$(HOMEBREW_TAP_REPO) && \
	for arch in $(SUPPORTED_ARCHS); do \
		echo "    - Processing $$arch..."; \
		if [ "$(DRY_RUN)" = "1" ]; then \
			echo "      [DRY_RUN] Would download: https://github.com/samzong/$(BINARY_NAME)/releases/download/v$(CLEAN_VERSION)/$(BINARY_NAME)_$${arch}.tar.gz"; \
			case "$$arch" in \
				Darwin_x86_64) DARWIN_AMD64_SHA="fake_sha_amd64" ;; \
				Darwin_arm64) DARWIN_ARM64_SHA="fake_sha_arm64" ;; \
				Linux_x86_64) LINUX_AMD64_SHA="fake_sha_linux_amd64" ;; \
				Linux_arm64) LINUX_ARM64_SHA="fake_sha_linux_arm64" ;; \
			esac; \
		else \
			echo "      - Downloading release archive..."; \
			curl -L $(CURL_VERBOSE) -SfO "https://github.com/samzong/$(BINARY_NAME)/releases/download/v$(CLEAN_VERSION)/$(BINARY_NAME)_$${arch}.tar.gz" || { echo "❌ Failed to download $$arch archive from https://github.com/samzong/$(BINARY_NAME)/releases/download/v$(CLEAN_VERSION)/$(BINARY_NAME)_$${arch}.tar.gz"; exit 1; }; \
			echo "      - Calculating SHA256..."; \
			sha=$$(shasum -a 256 "$(BINARY_NAME)_$${arch}.tar.gz" | cut -d' ' -f1); \
			case "$$arch" in \
				Darwin_x86_64) DARWIN_AMD64_SHA="$$sha"; echo "      ✓ Darwin AMD64 SHA: $$sha" ;; \
				Darwin_arm64) DARWIN_ARM64_SHA="$$sha"; echo "      ✓ Darwin ARM64 SHA: $$sha" ;; \
				Linux_x86_64) LINUX_AMD64_SHA="$$sha"; echo "      ✓ Linux AMD64 SHA: $$sha" ;; \
				Linux_arm64) LINUX_ARM64_SHA="$$sha"; echo "      ✓ Linux ARM64 SHA: $$sha" ;; \
			esac; \
		fi; \
	done; \
	\
	if [ "$(DRY_RUN)" = "1" ]; then \
		echo "==> [DRY_RUN] Would update formula with:"; \
		echo "    - Darwin AMD64 SHA: $$DARWIN_AMD64_SHA"; \
		echo "    - Darwin ARM64 SHA: $$DARWIN_ARM64_SHA"; \
		echo "    - Linux AMD64 SHA: $$LINUX_AMD64_SHA"; \
		echo "    - Linux ARM64 SHA: $$LINUX_ARM64_SHA"; \
		echo "    - Would commit and push changes"; \
		echo "    - Would create PR"; \
	else \
		echo "==> Updating formula file..."; \
		echo "    - Updating version to $(CLEAN_VERSION)"; \
		sed -i '' -e 's|version ".*"|version "$(CLEAN_VERSION)"|' $(FORMULA_FILE); \
		\
		echo "    - Updating URLs and checksums"; \
		sed -i '' \
			-e '/on_macos/,/end/ { \
				/if Hardware::CPU.arm?/,/else/ { \
					s|url ".*"|url "https://github.com/samzong/$(BINARY_NAME)/releases/download/v#{version}/$(BINARY_NAME)_Darwin_arm64.tar.gz"|; \
					s|sha256 ".*"|sha256 "'"$$DARWIN_ARM64_SHA"'"|; \
				}; \
				/else/,/end/ { \
					s|url ".*"|url "https://github.com/samzong/$(BINARY_NAME)/releases/download/v#{version}/$(BINARY_NAME)_Darwin_x86_64.tar.gz"|; \
					s|sha256 ".*"|sha256 "'"$$DARWIN_AMD64_SHA"'"|; \
				}; \
			}' \
			-e '/on_linux/,/end/ { \
				/if Hardware::CPU.arm?/,/else/ { \
					s|url ".*"|url "https://github.com/samzong/$(BINARY_NAME)/releases/download/v#{version}/$(BINARY_NAME)_Linux_arm64.tar.gz"|; \
					s|sha256 ".*"|sha256 "'"$$LINUX_ARM64_SHA"'"|; \
				}; \
				/else/,/end/ { \
					s|url ".*"|url "https://github.com/samzong/$(BINARY_NAME)/releases/download/v#{version}/$(BINARY_NAME)_Linux_x86_64.tar.gz"|; \
					s|sha256 ".*"|sha256 "'"$$LINUX_AMD64_SHA"'"|; \
				}; \
			}' $(FORMULA_FILE); \
		\
		echo "    - Checking for changes..."; \
		if ! git diff --quiet $(FORMULA_FILE); then \
			echo "==> Changes detected, creating pull request..."; \
			echo "    - Adding changes to git"; \
			git add $(FORMULA_FILE); \
			echo "    - Committing changes"; \
			git commit -m "chore: bump to $(VERSION)"; \
			echo "    - Pushing to remote"; \
			git push -u origin $(BRANCH_NAME); \
			echo "    - Preparing pull request data"; \
			pr_data=$$(jq -n \
				--arg title "chore: update $(BINARY_NAME) to $(VERSION)" \
				--arg body "Auto-generated PR\nSHAs:\n- Darwin(amd64): $$DARWIN_AMD64_SHA\n- Darwin(arm64): $$DARWIN_ARM64_SHA" \
				--arg head "$(BRANCH_NAME)" \
				--arg base "main" \
				'{title: $$title, body: $$body, head: $$head, base: $$base}'); \
			echo "    - Creating pull request"; \
			curl -X POST \
				-H "Authorization: token $(GH_PAT)" \
				-H "Content-Type: application/json" \
				https://api.github.com/repos/samzong/$(HOMEBREW_TAP_REPO)/pulls \
				-d "$$pr_data"; \
			echo "✅ Pull request created successfully"; \
		else \
			echo "❌ No changes detected in formula file"; \
			exit 1; \
		fi; \
	fi

	@echo "==> Cleaning up temporary files..."
	@rm -rf tmp
	@echo "✅ Homebrew formula update process completed"