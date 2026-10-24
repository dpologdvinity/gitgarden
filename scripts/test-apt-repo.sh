#!/usr/bin/env bash
# Smoke test for build-apt-repo.sh: builds a repository with a throwaway key, then uses the real
# apt tools (as a normal user, with private state directories) to check that the signature
# verifies and the package can be found and downloaded.
#
# Usage: scripts/test-apt-repo.sh <debs-dir>
set -euo pipefail

debs=$(cd "$1" && pwd)
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

export GNUPGHOME="$work/gnupg"
mkdir -m 700 "$GNUPGHOME"
gpg --batch --quiet --passphrase '' --quick-generate-key "gitgarden test <test@example.invalid>" ed25519 sign never
key=$(gpg --batch --list-keys --with-colons | awk -F: '/^fpr/ {print $10; exit}')

"$here/build-apt-repo.sh" "$debs" "$work/site" "$key"

# Point apt at the file:// repository, with state and cache inside the temp directory.
mkdir -p "$work/apt/lists/partial" "$work/apt/cache/archives/partial"
: > "$work/apt/status"
apt_opts=(
  -o "Dir::State=$work/apt"
  -o "Dir::State::lists=$work/apt/lists"
  -o "Dir::State::status=$work/apt/status"
  -o "Dir::Cache=$work/apt/cache"
  -o "Dir::Etc::sourcelist=$work/sources.list"
  -o "Dir::Etc::sourceparts=/dev/null"
  -o "Debug::NoLocking=1"
)
echo "deb [signed-by=$work/site/gitgarden.gpg] file://$work/site stable main" > "$work/sources.list"

apt-get "${apt_opts[@]}" update
apt-cache "${apt_opts[@]}" policy gitgarden | grep -q "Candidate: [0-9]"
(cd "$work" && apt-get "${apt_opts[@]}" download gitgarden)
ls "$work"/gitgarden_*.deb >/dev/null
echo "apt repository OK"
