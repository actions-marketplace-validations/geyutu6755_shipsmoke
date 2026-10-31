# Rust CLI example

Demonstrates shipsmoke as a release quality gate for a Rust CLI (`rustkit`,
built with cargo — no dependencies beyond std).

The CI workflow builds `rustkit` on all three operating systems, packages it
(`tar.gz` / `zip`), and verifies the package with shipsmoke: safe extraction,
binary identification from the real header, `--version` output assertions.
shipsmoke is language-agnostic: it never needs to know the toolchain.
