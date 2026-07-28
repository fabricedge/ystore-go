# Contributing

Thanks for your interest in contributing to `ystore-go`!

## Pull Request Process

1. Fork the repo, create a feature branch.
2. Write or update tests for your change.
3. Run `make test` — all tests must pass.
4. Run `make lint` — no vet warnings.
5. Keep commits small and descriptive. Use conventional commits:
   - `feat:` new feature
   - `fix:` bug fix
   - `docs:` documentation only
   - `refactor:` code change with no functional change
   - `test:` adding or fixing tests
6. Open a PR against `main` with a clear description of the change.

## Development Setup

```bash
git clone https://github.com/yourname/ystore-go.git
cd ystore-go
make build
```

**Prerequisites:**
- Go 1.24+
- ffmpeg (in PATH or at `/usr/bin/ffmpeg`)

## Code Style

- Follow `gofmt` formatting (default Go style).
- Error wrapping: `fmt.Errorf("context: %w", err)`.
- No `panic` except in `main()` or test helpers.
- CLI flags use long-form (`--input`, not `-i`).
- All exported symbols must have Go doc comments.

## Testing

- Tests live in `*_test.go` files alongside the package.
- Full pipeline tests require ffmpeg (skip gracefully if absent).
- Run `make test` to run all tests.

## License

By contributing, you agree that your contributions will be licensed under the
project's [MIT License](LICENSE).
