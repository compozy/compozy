# TODO: Finalizing AUR Automation for Compozy

After merging this Pull Request, the repository owner needs to perform these one-time setup steps to enable automatic updates to the Arch User Repository (AUR).

## 1. Create an AUR Account

If you don't already have one, create an account at [aur.archlinux.org](https://aur.archlinux.org/).

- **Username:** `compozy` (recommended) or your personal username.
- **SSH Public Key:** You will need to add a public key here (see Step 2).

## 2. Generate and Configure SSH Keys

GoReleaser needs an SSH key to push updates to the AUR.

1. **Generate a new keypair** (no passphrase):
   ```bash
   ssh-keygen -t ed25519 -f ~/.ssh/aur_compozy -N ""
   ```
2. **Add the Public Key to AUR:**
   - Copy the content of `~/.ssh/aur_compozy.pub`.
   - Log in to AUR -> My Account -> SSH Public Key -> Paste the key and Save.

3. **Add the Private Key to GitHub Secrets:**
   - Copy the content of the private key: `cat ~/.ssh/aur_compozy`.
   - In your GitHub repository: **Settings > Secrets and variables > Actions > New repository secret**.
   - **Name:** `AUR_KEY`
   - **Secret:** (Paste the entire private key block including BEGIN/END lines).

## 3. Claim the AUR Package (First Time Only)

The automation can update existing packages but cannot create them. You must perform the initial push manually:

```bash
# Clone the (currently empty) AUR repo
git clone ssh://aur@aur.archlinux.org/compozy-bin.git
cd compozy-bin

# Copy the PKGBUILD from this PR's aur-pkg directory
cp /path/to/compozy/aur-pkg/PKGBUILD .

# Generate the .SRCINFO file required by AUR
makepkg --printsrcinfo > .SRCINFO

# Commit and push to claim ownership
git add PKGBUILD .SRCINFO
git commit -m "Initial commit for compozy-bin"
git push origin master
```

## 4. Update .goreleaser.yml (If needed)

Ensure the `release.github.owner` in `.goreleaser.yml` matches the upstream organization (`compozy`).

## 5. Verify the Automation

The next time you push a version tag (e.g., `v0.1.7`), the GitHub Action will:

1. Build the binaries.
2. Create the GitHub Release.
3. Automatically update the AUR package with the new version and correct SHA256 checksums.

---

**Note:** If you want someone else (like @guifavretto) to maintain the package while you handle releases, you can add them as a **Co-maintainer** in the AUR web interface.
