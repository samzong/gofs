#!/bin/bash
set -euo pipefail

# Update Homebrew formula with new version
PROJECT_NAME="${PROJECT_NAME:-gofs}"
# Owner of the main project where releases live (can be different from tap owner)
PROJECT_OWNER="${PROJECT_OWNER:-samzong}"
HOMEBREW_TAP_REPO="${HOMEBREW_TAP_REPO:-homebrew-tap}" 
HOMEBREW_TAP_OWNER="${HOMEBREW_TAP_OWNER:-samzong}"
FORMULA_FILE="Formula/${PROJECT_NAME}.rb"
WORK_DIR="tmp"
# Absolute root of the repository when the script started
ROOT_DIR="$(pwd -P)"

VERSION=""
CLEAN_VERSION=""
BRANCH_NAME=""
DRY_RUN=0
VERBOSE=0

cleanup() {
    [[ $VERBOSE == 1 ]] && echo "Cleaning up..."
    rm -rf "$ROOT_DIR/$WORK_DIR"
}
trap cleanup EXIT

# Detect cross-platform sed in-place flag
setup_sed_inplace() {
    # GNU sed supports -i without argument; BSD sed (macOS) requires -i ''
    if sed --version >/dev/null 2>&1; then
        # GNU sed
        SED_INPLACE_CMD=("-i")
    else
        # BSD sed
        SED_INPLACE_CMD=("-i" "")
    fi
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS] <version>

Options:
  -d, --dry-run    Simulate without making changes
  -v, --verbose    Verbose output
  -h, --help       Show help

Environment Variables:
  GH_PAT                 GitHub token (required)
  PROJECT_NAME           Project name (default: gofs)
  PROJECT_OWNER          Project owner for releases (default: samzong)
  HOMEBREW_TAP_REPO      Tap repo (default: homebrew-tap)
  HOMEBREW_TAP_OWNER     Tap owner (default: samzong)

Examples:
  $0 v1.2.3
  $0 --dry-run v1.2.3

EOF
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dry-run) DRY_RUN=1; shift ;;
            -v|--verbose) VERBOSE=1; shift ;;
            -h|--help) usage; exit 0 ;;
            -*) echo "Error: Unknown option $1" >&2; exit 1 ;;
            *) 
                if [[ -n $VERSION ]]; then
                    echo "Error: Version already specified" >&2; exit 1
                fi
                VERSION="$1"; shift ;;
        esac
    done
    
    [[ -n $VERSION ]] || { echo "Error: Version required" >&2; exit 1; }
}

validate_prereqs() {
    echo "Checking prerequisites..."
    # Only require GH_PAT when not in dry-run mode
    if [[ $DRY_RUN == 0 ]]; then
        [[ -n ${GH_PAT:-} ]] || { echo "Error: GH_PAT required" >&2; exit 1; }
        # Prefer GitHub-provided token for current repo operations; fallback to PAT
        export GH_TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-$GH_PAT}}"
    fi
    
    for cmd in git curl shasum gh; do
        command -v "$cmd" >/dev/null || { echo "Error: $cmd not found" >&2; exit 1; }
        [[ $VERBOSE == 1 ]] && echo "Found: $cmd"
    done
}

validate_version() {
    [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+.*$ ]] || {
        echo "Error: Invalid version format: $VERSION (expected: v1.2.3)" >&2
        exit 1
    }
    
    CLEAN_VERSION=$(echo "$VERSION" | sed 's/^v//' | cut -d'-' -f1)
    BRANCH_NAME="update-${PROJECT_NAME}-${CLEAN_VERSION}"
    
    [[ $VERBOSE == 1 ]] && echo "Version: $VERSION -> $CLEAN_VERSION"
}

check_release() {
    if [[ $DRY_RUN == 1 ]]; then
        echo "[DRY RUN] Would check release: $VERSION"
        return
    fi
    
    echo "Checking if release $VERSION exists..."
    gh release view "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" >/dev/null || {
        echo "Error: Release $VERSION not found" >&2
        exit 1
    }
}

setup_workspace() {
    rm -rf "$ROOT_DIR/$WORK_DIR"
    mkdir -p "$ROOT_DIR/$WORK_DIR"
}

clone_repo() {
    if [[ $DRY_RUN == 1 ]]; then
        echo "[DRY RUN] Would clone: ${HOMEBREW_TAP_OWNER}/${HOMEBREW_TAP_REPO}"
        mkdir -p "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}"
        return
    fi
    
    echo "Cloning homebrew tap..."
    git clone "https://${GH_PAT}@github.com/${HOMEBREW_TAP_OWNER}/${HOMEBREW_TAP_REPO}.git" "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}"
    # Configure bot identity for commits
    git -C "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}" config user.name "github-actions[bot]"
    git -C "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
    # Idempotent branch creation/update
    git -C "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}" checkout -B "$BRANCH_NAME"
}

calculate_checksums() {
    cd "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}"
    echo "Calculating checksums..."

    if [[ $DRY_RUN == 1 ]]; then
        DARWIN_AMD64_SHA="fake_sha_darwin-amd64"
        DARWIN_ARM64_SHA="fake_sha_darwin-arm64"
        LINUX_AMD64_SHA="fake_sha_linux-amd64"
        LINUX_ARM64_SHA="fake_sha_linux-arm64"
        return
    fi

    # Discover asset download URLs from the release instead of guessing names
    # Requires gh CLI with access token (GH_TOKEN/GITHUB_TOKEN).
    local assets
    if ! assets=$(gh release view "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" --json assets --jq '.assets[] | "\(.name)\t\(.browser_download_url)"'); then
        echo "Error: Failed to list release assets for $VERSION" >&2
        exit 1
    fi

    local darwin_amd64_url="" darwin_arm64_url="" linux_amd64_url="" linux_arm64_url=""
    local darwin_amd64_name="" darwin_arm64_name="" linux_amd64_name="" linux_arm64_name=""

    # Select appropriate assets by name patterns (case-insensitive)
    while IFS=$'\t' read -r name url; do
        lower_name=$(echo "$name" | tr '[:upper:]' '[:lower:]')
        # Only consider archive assets
        case "$lower_name" in
            *.tar.gz|*.tgz) : ;; # ok
            *) continue ;;
        esac
        if [[ "$lower_name" == *darwin* && ( "$lower_name" == *amd64* || "$lower_name" == *x86_64* ) ]]; then
            darwin_amd64_url="$url"; darwin_amd64_name="$name"
            [[ $VERBOSE == 1 ]] && echo "Matched darwin/amd64 asset: $name"
        elif [[ "$lower_name" == *darwin* && "$lower_name" == *arm64* ]]; then
            darwin_arm64_url="$url"; darwin_arm64_name="$name"
            [[ $VERBOSE == 1 ]] && echo "Matched darwin/arm64 asset: $name"
        elif [[ "$lower_name" == *linux* && ( "$lower_name" == *amd64* || "$lower_name" == *x86_64* ) ]]; then
            linux_amd64_url="$url"; linux_amd64_name="$name"
            [[ $VERBOSE == 1 ]] && echo "Matched linux/amd64 asset: $name"
        elif [[ "$lower_name" == *linux* && "$lower_name" == *arm64* ]]; then
            linux_arm64_url="$url"; linux_arm64_name="$name"
            [[ $VERBOSE == 1 ]] && echo "Matched linux/arm64 asset: $name"
        fi
    done <<< "$assets"

    # Ensure we found all required assets
    [[ -n "$darwin_amd64_url" ]] || { echo "Error: Missing darwin/amd64 asset in release" >&2; exit 1; }
    [[ -n "$darwin_arm64_url" ]] || { echo "Error: Missing darwin/arm64 asset in release" >&2; exit 1; }
    [[ -n "$linux_amd64_url" ]] || { echo "Error: Missing linux/amd64 asset in release" >&2; exit 1; }
    [[ -n "$linux_arm64_url" ]] || { echo "Error: Missing linux/arm64 asset in release" >&2; exit 1; }

    # Prefer gh to download assets by exact name to avoid URL/CDN issues
    local download_dir
    download_dir="${ROOT_DIR}/${WORK_DIR}/downloads"
    mkdir -p "$download_dir"

    # We use --pattern with the exact name and force into our download dir
    if ! gh release download "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" --pattern "$darwin_amd64_name" --dir "$download_dir" --clobber >/dev/null; then
        echo "Error: Failed to download asset: $darwin_amd64_name" >&2; exit 1; fi
    if ! gh release download "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" --pattern "$darwin_arm64_name" --dir "$download_dir" --clobber >/dev/null; then
        echo "Error: Failed to download asset: $darwin_arm64_name" >&2; exit 1; fi
    if ! gh release download "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" --pattern "$linux_amd64_name" --dir "$download_dir" --clobber >/dev/null; then
        echo "Error: Failed to download asset: $linux_amd64_name" >&2; exit 1; fi
    if ! gh release download "$VERSION" --repo "${PROJECT_OWNER}/${PROJECT_NAME}" --pattern "$linux_arm64_name" --dir "$download_dir" --clobber >/dev/null; then
        echo "Error: Failed to download asset: $linux_arm64_name" >&2; exit 1; fi

    DARWIN_AMD64_SHA=$(shasum -a 256 "$download_dir/$darwin_amd64_name" | cut -d' ' -f1)
    DARWIN_ARM64_SHA=$(shasum -a 256 "$download_dir/$darwin_arm64_name" | cut -d' ' -f1)
    LINUX_AMD64_SHA=$(shasum -a 256 "$download_dir/$linux_amd64_name" | cut -d' ' -f1)
    LINUX_ARM64_SHA=$(shasum -a 256 "$download_dir/$linux_arm64_name" | cut -d' ' -f1)

    [[ $VERBOSE == 1 ]] && {
        echo "darwin/amd64 sha: $DARWIN_AMD64_SHA"
        echo "darwin/arm64 sha: $DARWIN_ARM64_SHA"
        echo "linux/amd64 sha:  $LINUX_AMD64_SHA"
        echo "linux/arm64 sha:  $LINUX_ARM64_SHA"
    }
}

update_formula() {
    if [[ $DRY_RUN == 1 ]]; then
        echo "[DRY RUN] Would update formula with checksums:"
        echo "  Darwin AMD64: $DARWIN_AMD64_SHA"
        echo "  Darwin ARM64: $DARWIN_ARM64_SHA"
        echo "  Linux AMD64: $LINUX_AMD64_SHA"
        echo "  Linux ARM64: $LINUX_ARM64_SHA"
        return
    fi
    
    cd "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}"
    echo "Updating formula..."
    
    # Ensure formula exists
    if [[ ! -f "$FORMULA_FILE" ]]; then
        echo "Error: Formula file not found: $FORMULA_FILE" >&2
        exit 1
    fi
    
    setup_sed_inplace
    
    # Update version
    sed "${SED_INPLACE_CMD[@]}" -E "s|version \".*\"|version \"$CLEAN_VERSION\"|" "$FORMULA_FILE"
    
    # Normalize asset URLs to use v#{version} if hard-coded
    sed "${SED_INPLACE_CMD[@]}" -E "s|(/releases/download/)v[0-9][0-9\.]*|\1v\#\{version\}|g" "$FORMULA_FILE"
    
    # Update checksums - simplified sed commands
    sed "${SED_INPLACE_CMD[@]}" \
        -e '/Darwin_arm64/,/sha256/ s|sha256 \".*\"|sha256 \"'$DARWIN_ARM64_SHA'\"|' \
        -e '/Darwin_x86_64/,/sha256/ s|sha256 \".*\"|sha256 \"'$DARWIN_AMD64_SHA'\"|' \
        -e '/Linux_arm64/,/sha256/ s|sha256 \".*\"|sha256 \"'$LINUX_ARM64_SHA'\"|' \
        -e '/Linux_x86_64/,/sha256/ s|sha256 \".*\"|sha256 \"'$LINUX_AMD64_SHA'\"|' \
        "$FORMULA_FILE"
}

create_pr() {
    if [[ $DRY_RUN == 1 ]]; then
        echo "[DRY RUN] Would commit and create PR"
        return
    fi
    
    cd "${ROOT_DIR}/${WORK_DIR}/${HOMEBREW_TAP_REPO}"
    
    if git diff --quiet "$FORMULA_FILE"; then
        echo "No changes detected in formula; nothing to commit."
        return 0
    fi
    
    echo "Creating pull request..."
    git add "$FORMULA_FILE"
    git commit -m "chore: bump $PROJECT_NAME to $VERSION"
    git push -u origin "$BRANCH_NAME"
    
    GH_TOKEN="$GH_PAT" gh pr create \
        --repo "${HOMEBREW_TAP_OWNER}/${HOMEBREW_TAP_REPO}" \
        --title "chore: update $PROJECT_NAME to $VERSION" \
        --body "Auto-generated PR to update $PROJECT_NAME to $VERSION

## Checksums
- Darwin (AMD64): $DARWIN_AMD64_SHA
- Darwin (ARM64): $DARWIN_ARM64_SHA  
- Linux (AMD64): $LINUX_AMD64_SHA
- Linux (ARM64): $LINUX_ARM64_SHA" \
        --head "$BRANCH_NAME" \
        --base "main" || { echo "Error: Failed to create PR" >&2; exit 1; }
}

main() {
    parse_args "$@"
    validate_prereqs
    validate_version
    check_release
    setup_workspace
    clone_repo
    calculate_checksums
    update_formula
    create_pr
    echo "✅ Homebrew formula updated successfully"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi