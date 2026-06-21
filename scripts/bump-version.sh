#!/bin/bash

# Bump version script for makemigrations
# Usage: ./scripts/bump-version.sh [patch|minor|major] [--dry-run] [--allow-dirty]

set -e

# Default values
VERSION_PART="patch"
DRY_RUN=""
ALLOW_DIRTY=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        patch|minor|major)
            VERSION_PART="$1"
            shift
            ;;
        --dry-run)
            DRY_RUN="--dry-run"
            shift
            ;;
        --allow-dirty)
            ALLOW_DIRTY="--allow-dirty"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [patch|minor|major] [--dry-run] [--allow-dirty]"
            echo ""
            echo "Arguments:"
            echo "  patch|minor|major  Version part to bump (default: patch)"
            echo "  --dry-run          Show what would be done without making changes"
            echo "  --allow-dirty      Allow bumping even if git working directory is dirty"
            echo ""
            echo "Examples:"
            echo "  $0                     # Bump patch version"
            echo "  $0 minor               # Bump minor version"
            echo "  $0 major --dry-run     # Preview major version bump"
            echo "  $0 patch --allow-dirty # Bump patch even with uncommitted changes"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Check if bumpversion is installed
if ! command -v bumpversion &> /dev/null; then
    echo "Error: bumpversion is not installed."
    echo "Install with: pip install bumpversion"
    exit 1
fi

# Check if we're in the project root
if [[ ! -f ".bumpversion.cfg" ]]; then
    echo "Error: .bumpversion.cfg not found. Please run this script from the project root."
    exit 1
fi

# Get current version
CURRENT_VERSION=$(grep "current_version" .bumpversion.cfg | cut -d' ' -f3)

# Show what we're about to do
echo "Current version: $CURRENT_VERSION"
echo "Bumping $VERSION_PART version..."

if [[ -n "$DRY_RUN" ]]; then
    echo "DRY RUN MODE - No changes will be made"
fi

# Run bumpversion
if [[ -n "$ALLOW_DIRTY" ]]; then
    echo "Warning: Allowing dirty working directory"
fi

bumpversion $DRY_RUN $ALLOW_DIRTY --verbose $VERSION_PART

if [[ -z "$DRY_RUN" ]]; then
    # Get new version
    NEW_VERSION=$(grep "current_version" .bumpversion.cfg | cut -d' ' -f3)
    echo ""
    echo "✅ Version bumped successfully!"
    echo "   Old version: $CURRENT_VERSION"
    echo "   New version: $NEW_VERSION"
    echo "   Git tag: v$NEW_VERSION"
    echo ""
    echo "Next steps:"
    echo "1. Review the changes: git show"
    echo "2. Push to trigger release: git push origin main --tags"
else
    echo ""
    echo "This was a dry run. Use the command without --dry-run to actually bump the version."
fi