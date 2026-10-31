# C CLI example

Demonstrates shipsmoke as a release quality gate for a plain C tool
(`ckit`, compiled with `cc` on Linux/macOS and `gcc` on Windows runners).

The CI workflow compiles `ckit` on all three operating systems, packages it,
and verifies the package with shipsmoke — proving the tool works for
artifacts that come from any build system, not just Go.
