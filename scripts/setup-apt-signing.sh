#!/usr/bin/env bash
# One-time setup for the signed apt repository. Run it once as the repository owner:
#
#   scripts/setup-apt-signing.sh
#
# It (1) creates a dedicated signing key in a temporary keyring, (2) stores the private key as the
# repository secret APT_GPG_PRIVATE_KEY, (3) saves a backup copy under ~/.config/gitgarden
# (mode 600, outside the repository), and (4) turns on GitHub Pages with "GitHub Actions" as source.
# Needs the GitHub CLI (`gh`) logged in with admin rights on the repository.
set -euo pipefail

repo=${1:-dpologdvinity/gitgarden}
backup_dir="$HOME/.config/gitgarden"
backup="$backup_dir/apt-signing-key.asc"

if [ -e "$backup" ]; then
  echo "refusing to overwrite the existing backup $backup" >&2
  exit 1
fi

umask 077
GNUPGHOME=$(mktemp -d)
export GNUPGHOME
trap 'rm -rf "$GNUPGHOME"' EXIT

gpg --batch --quiet --passphrase '' --quick-generate-key \
  "gitgarden apt repository <darcykaitlynb@gmail.com>" ed25519 sign never
key=$(gpg --batch --list-keys --with-colons | awk -F: '/^fpr/ {print $10; exit}')

mkdir -p "$backup_dir"
gpg --batch --armor --export-secret-keys "$key" > "$backup"
gh secret set APT_GPG_PRIVATE_KEY --repo "$repo" < "$backup"

# 409/422 mean Pages already exists; switch its source instead.
gh api -X POST "repos/$repo/pages" -f build_type=workflow >/dev/null 2>&1 \
  || gh api -X PUT "repos/$repo/pages" -f build_type=workflow >/dev/null

echo "Done. Signing key fingerprint: $key"
echo "Private key backup: $backup (keep it safe; it is also stored as a repository secret)."
echo "Publish the repository with: gh workflow run apt-repo.yml --repo $repo"
