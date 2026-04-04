# AUR Support for Compozy

This directory contains PKGBUILD templates for making Compozy available in the Arch User Repository (AUR).

## Files

- `PKGBUILD`: Stable binary release package (`compozy-bin`). Recommended for most users as it's faster to install.
- `PKGBUILD-src`: Stable source release package (`compozy`). Builds from source using Go.

## How to use

1.  Create a new repository on AUR (e.g., `compozy-bin`).
2.  Clone it locally.
3.  Copy the relevant `PKGBUILD` from this directory to your AUR repository.
4.  Run `makepkg --printsrcinfo > .SRCINFO`.
5.  Commit and push to AUR.

## Maintainer Note

Replace the `Maintainer` line with your own name and email if you are the one uploading it to AUR.
Currently set to `Compozy Team`.
