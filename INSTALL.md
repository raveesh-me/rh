# Installing rh

`rh` is a single static binary with no config file. Pick one:

## Prebuilt binary (macOS / Linux)

```sh
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = x86_64 ] && ARCH=amd64; [ "$ARCH" = aarch64 ] && ARCH=arm64
curl -fsSL "https://github.com/raveesh-me/rh/releases/latest/download/rh_${OS}_${ARCH}.tar.gz" \
  | tar -xz -C /tmp rh
sudo install -m 0755 /tmp/rh /usr/local/bin/rh   # or: install -m 0755 /tmp/rh ~/.local/bin/rh
rh --version
```

Assets on each [release](https://github.com/raveesh-me/rh/releases): `rh_darwin_arm64`, `rh_darwin_amd64`, `rh_linux_arm64`, `rh_linux_amd64` (`.tar.gz`), plus `checksums.txt`.

If you downloaded through a browser on macOS and Gatekeeper blocks it:

```sh
xattr -d com.apple.quarantine /usr/local/bin/rh
```

## With Go (1.26+)

```sh
go install github.com/raveesh-me/rh@latest
```

Installs to `$(go env GOPATH)/bin` — make sure that's on your `PATH`. `rh --version` prints `dev` for this route.

## From source

```sh
git clone git@github.com:raveesh-me/rh.git && cd rh
go build -o rh . && install -m 0755 rh ~/.local/bin/rh
```

## Cutting a release (maintainers)

Push a semver tag; the `release` workflow runs tests and GoReleaser publishes the binaries:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

## Updating

```sh
rh update        # sudo rh update if installed in /usr/local/bin
```

Downloads the latest release for your OS/arch, verifies it against `checksums.txt`, and swaps the binary in place. Works for any install route above (a `go install` / source build reports version `dev` and is always replaced).

## Uninstall

```sh
rm "$(command -v rh)"
```
