# Homebrew Update Script Configuration
HOMEBREW_SCRIPT := ./scripts/update-homebrew.sh

# Export environment variables for the script
export PROJECT_NAME := $(PROJECT_NAME)
export HOMEBREW_TAP_REPO := homebrew-tap
export HOMEBREW_TAP_OWNER := samzong

##@ Homebrew
.PHONY: update-homebrew
update-homebrew: ## Update Homebrew formula with new version
	@echo "$(BLUE)Starting Homebrew formula update...$(NC)"
	@if [ -z "$(GH_PAT)" ]; then \
		echo "$(RED)❌ Error: GH_PAT environment variable is required$(NC)"; \
		echo "$(YELLOW)Please set GH_PAT with a GitHub Personal Access Token$(NC)"; \
		exit 1; \
	fi
	@if [ ! -x "$(HOMEBREW_SCRIPT)" ]; then \
		echo "$(RED)❌ Error: Update script not found or not executable: $(HOMEBREW_SCRIPT)$(NC)"; \
		exit 1; \
	fi
	@$(HOMEBREW_SCRIPT) "$(VERSION)"

.PHONY: update-homebrew-dry-run
update-homebrew-dry-run: ## Simulate Homebrew formula update (dry run)
	@echo "$(YELLOW)Running Homebrew formula update in dry-run mode...$(NC)"
	@if [ ! -x "$(HOMEBREW_SCRIPT)" ]; then \
		echo "$(RED)❌ Error: Update script not found or not executable: $(HOMEBREW_SCRIPT)$(NC)"; \
		exit 1; \
	fi
	@DRY_RUN=1 $(HOMEBREW_SCRIPT) "$(VERSION)"

.PHONY: update-homebrew-verbose
update-homebrew-verbose: ## Update Homebrew formula with verbose output
	@echo "$(BLUE)Starting Homebrew formula update with verbose output...$(NC)"
	@if [ -z "$(GH_PAT)" ]; then \
		echo "$(RED)❌ Error: GH_PAT environment variable is required$(NC)"; \
		exit 1; \
	fi
	@if [ ! -x "$(HOMEBREW_SCRIPT)" ]; then \
		echo "$(RED)❌ Error: Update script not found or not executable: $(HOMEBREW_SCRIPT)$(NC)"; \
		exit 1; \
	fi
	@VERBOSE=1 $(HOMEBREW_SCRIPT) "$(VERSION)"