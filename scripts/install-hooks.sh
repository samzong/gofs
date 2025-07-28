#!/bin/bash
# Git hooks installation script

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Installing Git hooks...${NC}"

# Check if we're in a Git repository
if [ ! -d ".git" ]; then
    echo -e "${RED}Error: Current directory is not a Git repository${NC}"
    exit 1
fi

# Create hooks directory (if it doesn't exist)
mkdir -p .git/hooks

# Copy pre-commit hook
if [ -f ".githooks/pre-commit" ]; then
    cp .githooks/pre-commit .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit
    echo -e "${GREEN}Pre-commit hook installed${NC}"
else
    echo -e "${RED}Error: .githooks/pre-commit file does not exist${NC}"
    exit 1
fi

echo -e "${GREEN}Git hooks installation completed!${NC}"
echo -e "${BLUE}Code quality checks will now run automatically on every git commit${NC}"
echo -e "${BLUE}To skip checks, use: git commit --no-verify${NC}"