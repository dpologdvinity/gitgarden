#!/usr/bin/env bash
# Builds a signed apt repository from a directory of .deb files.
#
# Usage: scripts/build-apt-repo.sh <debs-dir> <out-dir> <gpg-key-id>
#
# The key must already be in the current GPG keyring (set GNUPGHOME to use another one).
# The output is a static site that can be served from any web server (we use GitHub Pages):
#   <out>/pool/main/g/gitgarden/*.deb          the packages
#   <out>/dists/stable/main/binary-<arch>/     Packages indexes
#   <out>/dists/stable/{Release,InRelease,Release.gpg}   signed metadata
#   <out>/gitgarden.gpg                        public key in the binary format apt expects
set -euo pipefail

if [ $# -ne 3 ]; then
  echo "usage: $0 <debs-dir> <out-dir> <gpg-key-id>" >&2
  exit 2
fi
debs=$1
out=$2
key=$3
arches="amd64 arm64"

mkdir -p "$out/pool/main/g/gitgarden"
cp "$debs"/*.deb "$out/pool/main/g/gitgarden/"

cd "$out"
for arch in $arches; do
  dir="dists/stable/main/binary-$arch"
  mkdir -p "$dir"
  # --arch keeps only this architecture (and arch-independent packages) in the index.
  apt-ftparchive --arch "$arch" packages pool > "$dir/Packages"
  gzip -9 -k -f "$dir/Packages"
done

apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=gitgarden \
  -o APT::FTPArchive::Release::Label=gitgarden \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures="$arches" \
  -o APT::FTPArchive::Release::Components=main \
  release dists/stable > dists/stable/Release

gpg_sign=(gpg --batch --yes --pinentry-mode loopback --default-key "$key")
"${gpg_sign[@]}" --clearsign -o dists/stable/InRelease dists/stable/Release
"${gpg_sign[@]}" --armor --detach-sign -o dists/stable/Release.gpg dists/stable/Release
gpg --batch --yes --export "$key" > gitgarden.gpg

cat > index.html <<'HTML'
<!doctype html>
<title>gitgarden apt repository</title>
<h1>gitgarden apt repository</h1>
<p>Install instructions: <a href="https://github.com/dpologdvinity/gitgarden#apt-debian-ubuntu">README</a></p>
HTML
