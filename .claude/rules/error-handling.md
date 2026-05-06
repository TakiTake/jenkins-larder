Always handle errors. If a function returns an `error`, check it. Never discard with `_ =`.

- Source code: log or return the error
- Tests with `t` available: use `t.Fatal(err)` or `t.Fatalf`
- HTTP handler mocks without `t`: check error and return early

A pre-commit hook runs `golangci-lint` (with errcheck) on every commit.
