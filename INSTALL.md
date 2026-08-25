# Installing cs-lint

`cs-lint` is a single static binary. It needs no runtime, no interpreter and no
packages: you put one file on the path and run it. It reads a repository and
writes nothing, so nothing on your machine changes when you use it.

Once it runs, [`MANUAL.md`](MANUAL.md) has the full surface and
[`README.md`](README.md) shows what each linter is for.

## 1. Get it

Three routes. Take the first one that fits.

### With the Go toolchain

The shortest route, and the one that keeps the tool current:

```bash
go install github.com/codesweep-ai/lint/cmd/cs-lint@latest
```

This needs **Go 1.26 or newer**. The binary lands in `$(go env GOPATH)/bin`,
which is `~/go/bin` unless you have moved it. Put that directory on your `PATH`
if it is not already there:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

A project that wants the same version for everybody pins it in its own
`go.mod` instead, which records the version where a reviewer sees it change:

```bash
go get -tool github.com/codesweep-ai/lint/cmd/cs-lint@latest
go tool cs-lint prose
```

### From a release archive

Take this route on a machine with no Go toolchain. Replace `VERSION` with the release you
want, and `linux_amd64` with your platform:

```bash
VERSION=0.1.0
curl -fsSLO https://github.com/codesweep-ai/lint/releases/download/v${VERSION}/cs-lint_${VERSION}_linux_amd64.tar.gz
curl -fsSLO https://github.com/codesweep-ai/lint/releases/download/v${VERSION}/checksums.txt
sha256sum --check --ignore-missing checksums.txt
tar -xzf cs-lint_${VERSION}_linux_amd64.tar.gz
install -m 0755 cs-lint ~/.local/bin/cs-lint
```

The checksum file is signed. To verify the signature as well, with
[cosign](https://docs.sigstore.dev/cosign/system_config/installation/)
installed:

```bash
curl -fsSLO https://github.com/codesweep-ai/lint/releases/download/v${VERSION}/checksums.txt.sig
curl -fsSLO https://github.com/codesweep-ai/lint/releases/download/v${VERSION}/checksums.txt.pem
cosign verify-blob checksums.txt \
  --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  --certificate-identity-regexp 'https://github\.com/codesweep-ai/lint/\.github/workflows/release\.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Each archive also ships a software bill of materials as
`<archive>.sbom.json`.

### From a clone

For working on cs-lint itself, or for a platform the releases do not cover:

```bash
git clone https://github.com/codesweep-ai/lint
cd lint
make install            # builds, then copies into ~/.local/bin
```

`make install` puts the binary in `$(PREFIX)/bin`, where `PREFIX` defaults to
`$(HOME)/.local`. Override it for a system-wide install:

```bash
sudo make install PREFIX=/usr/local
```

`make uninstall` removes it again, with the same `PREFIX`.

## 2. Host prerequisites

The binary itself needs nothing. Some checks shell out to other programs, and
each one reports a skip rather than a failure when its program is absent. A
machine without them still gets a useful run.

| Program | Which checks need it | Without it |
|---|---|---|
| `git` | the leak scan, and every history rule | those rules skip |
| `goreleaser` | the release-manifest check | that rule skips |
| `actionlint` | the workflow check | that rule skips |
| `gh` | `oss --online` | those rules skip |
| `cs-ledger` | the ledger rules, in a repository that keeps one | those rules skip |
| `cosign` | verifying a release you downloaded | you cannot check the signature |

Install what you want on Fedora or RHEL:

```bash
sudo dnf install -y git
go install github.com/goreleaser/goreleaser/v2@latest
go install github.com/rhysd/actionlint/cmd/actionlint@latest
```

On Debian or Ubuntu, run:

```bash
sudo apt-get install -y git
go install github.com/goreleaser/goreleaser/v2@latest
go install github.com/rhysd/actionlint/cmd/actionlint@latest
```

On macOS, run:

```bash
brew install git goreleaser actionlint gh
```

## 3. First-run setup

There is none. cs-lint keeps no state, writes no cache, and reads no file
outside the repository you point it at.

A repository you want to tune gets a `.cs-lint.yaml` at its root. A repository
with nothing to tune needs no file at all. [`MANUAL.md`](MANUAL.md) documents
every key.

## 4. Verify the installation

Check that the binary runs and reports its version:

```console
$ cs-lint version
cs-lint …
```

The elision stands for the version itself, which depends on how the binary was
built: a release prints its tag, and a build from a clone prints what `git
describe` gives.

Then point it at a repository and ask what it would check, which touches
nothing:

```bash
cd ~/code/my-project
cs-lint prose --list
```

That prints the Markdown files a prose run would read. If the list is empty or
carries files that are not this project's prose, the `docs.prose.skipExtra` key
is what fixes it.

Finally, run one linter for real:

```bash
cs-lint prose
```

It exits 0 when it found nothing, 1 when it found something, and 2 when it
could not run at all. Nothing is written either way.

## 5. Wire it into a project

The value is in the gate, not in the command you remember to type. Add a target
per linter, and fold them into the one command a contributor already runs
before pushing:

```make
## docs: check the prose, and the references the documents make
docs:
	cs-lint prose
	cs-lint refs

## oss: check that this repo is in a shape it can be published in
oss:
	cs-lint oss

## surface: check the docs against the binary, the code and the build
surface: build
	cs-lint surface

check: fmt-check vet lint test docs oss surface
```

`prose` and `refs` read the tracked tree and nothing else, so they answer
first and need no build. `surface` depends on `build` because every check in it
asks the binary itself.

Then give each linter its own CI job, rather than burying it behind the test
matrix. A linter answers in seconds and reports even when the tests are
failing:

```yaml
  docs:
    name: docs prose and refs
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: go install github.com/codesweep-ai/lint/cmd/cs-lint@latest
      - run: make docs
```

The readiness job needs one more thing, because the history rules read the
history and the default checkout is shallow:

```yaml
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0
```

Prove the gate works before you trust it. Add a deliberate violation, confirm a
non-zero exit, and restore the file:

```bash
echo "The the word is written twice." >> README.md
cs-lint prose; echo "exit $?"     # expect 1
git checkout -- README.md
```

## Shell completion

`cs-lint completion` writes a completion script for `bash`, `zsh`, `fish` or
`powershell`. Load it from your shell's startup file:

```bash
cs-lint completion bash > ~/.local/share/bash-completion/completions/cs-lint
```

```bash
cs-lint completion zsh > "${fpath[1]}/_cs-lint"
```

Run `cs-lint completion --help` for the per-shell instructions.

## Upgrading

With the Go toolchain, re-run the install command; it replaces the binary in
place:

```bash
go install github.com/codesweep-ai/lint/cmd/cs-lint@latest
```

From a release archive, fetch the new one and replace the old binary. From a
clone, run `git pull && make install`.

New versions gain checks, so a repository that passed on one version may report
findings on the next. Pin the version in CI if you would rather adopt new checks
deliberately.

## Removing it

```bash
make uninstall                     # a clone install
rm ~/go/bin/cs-lint                # a `go install` install
```

There is nothing else to remove: no configuration outside the repositories you
added it to, no cache, and no state.
