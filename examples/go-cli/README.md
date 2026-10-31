# Go CLI example

Demonstrates shipsmoke as a release quality gate for a Go CLI (`gokit`).

The CI workflow (`.github/workflows/example-go-cli.yml` at the repository root — GitHub only registers workflows from the root `.github/workflows/`):

1. builds `shipsmoke` itself from the checkout,
2. builds `gokit` and packages it — `tar.gz` on Linux/macOS, `zip` on Windows,
3. runs `shipsmoke check --config .shipsmoke.yml`, which:
   - extracts the package safely,
   - identifies the entry binary from its real header (ELF/PE/Mach-O),
   - runs `--version` and `--help` checks with output assertions,
   - reports the non-matching artifact as SKIP with a reason.

`.shipsmoke.yml` covers both packaging styles in one config, gated by
`runs_on`. Run it locally the same way after `go build`.

This is the same pattern you would use against your own project's `dist/`
directory — swap the paths and check args.
