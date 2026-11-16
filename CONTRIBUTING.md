# Contributing to IceDash

Thanks for helping. Bug reports, ideas, translations and pull requests are all welcome.

## Reporting a bug

[Open an issue](https://github.com/HSinghHira/IceDash/issues/new) with:

- your system and version (ZimaOS or CasaOS), and IceDash's version (IceDash menu → About);
- what you did, what you expected, and what happened instead;
- if you can, IceDash's log from around that time: `journalctl -u icedash --since '-10m' --no-pager` on ZimaOS (as root), or `journalctl -u icedash` on CasaOS.

Please don't post tokens, passwords or app credentials in an issue. For a security problem, open an issue that only asks for a private contact, without the details.

## Asking for a feature

Open an issue that says what you want to do and why. IceDash supports ZimaOS and CasaOS only, so features for other systems are out of scope.

## Working on the code

You need Go (the version in `go.mod`), Node.js 24, git and make. Building the ZimaOS image needs Linux with `squashfs-tools`; everything else works on Windows and macOS too.

```sh
make check      # go vet, Go tests and the web tests: run this before every pull request
make preview    # the desktop with sample data, at the address it prints
make site       # the website, at http://localhost:8000
```

- **Backend:** Go, in `cmd/icedash` and `internal/`. Run `gofmt` (CI rejects unformatted code). `cmd/icedash/main.go` is Linux-only, so also run `GOOS=linux go vet ./...` on Windows or macOS.
- **Frontend:** Vue and TypeScript in `web/`. Run the web tests with `make web-test`.
- **Translations:** the English strings are in `web/src/strings.ts` and the others in `web/src/i18n/`. A test checks that every language has every key, so add a new key to all of them.
- **Website:** `site/` (templates, styles and the content lists in `site/build.py`). The README pictures come from the same content: after changing it, run `python scripts/readme_svgs.py` (needs `pillow` and `fonttools`).

## Pull requests

- Keep each pull request to one change, and say what it does and how you tested it.
- Add or update tests for what you change.
- Nothing may change ZimaOS or CasaOS system files. IceDash stays removable, and its uninstall puts things back as they were.
- `make check` must pass. CI runs it, and builds the images, on every pull request.

Releases are made by the maintainer from version tags (`vX.Y.Z`) only. A plain push never reaches users.

## License

By contributing, you agree that your work is released under the [MIT License](LICENSE).

## lintux-modkit stays on v0.1.3

`lintux-modkit.ref` pins v0.1.3. From v0.1.4, its `auth.Verifier.Verify` refuses every role but `admin`, and tokens with no role claim (`ErrNotAdmin`, 403 `auth_forbidden`). IceDash lets ZimaOS `user` accounts in read-only, and CasaOS tokens carry no role, so v0.1.4 would lock out ZimaOS sub-accounts and every CasaOS sign-in. Don't bump the pin until IceDash checks roles itself, or modkit offers a way to let other roles through.
