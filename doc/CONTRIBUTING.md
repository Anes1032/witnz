# Contributing to Witnz

Thank you for your interest in contributing to Witnz! We welcome contributions from the community.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up your development environment** following the [README](README.md#development)
3. **Create a branch** from `main` for your changes
4. **Make your changes** following our guidelines below
5. **Submit a Pull Request** to the `main` branch

## Development Guidelines

### Code Style

- All code and comments must be in **English**
- Follow standard **Go conventions** (use `gofmt`, `golint`)
- Keep code comments to an **absolute minimum** - prefer self-documenting code
- Write clear variable and function names instead of explanatory comments

### Testing Requirements

All pull requests must include tests:

```bash
# Run tests before submitting
make test

# Ensure all tests pass
go test ./internal/... -v
```

- Write **table-driven tests** for new functionality
- Maintain or improve test coverage
- Test both success and error cases

### Commit Messages

Write clear, concise commit messages:

```
Add state integrity verification for append-only tables

- Implement Merkle tree calculation
- Add periodic verification scheduler
- Include tampering detection alerts
```

**Format:**
- First line: Brief summary (50 chars or less)
- Blank line
- Detailed explanation if needed (wrap at 72 chars)

### Pull Request Process

1. **Update documentation** if you change functionality
2. **Add tests** for new features or bug fixes
3. **Ensure CI passes** - all tests must pass before merge
4. **Request review** - PRs require maintainer approval before merge
5. **Address feedback** - respond to review comments promptly

### Branch Protection Rules

The following branches are protected:

- **`main`**: Requires 1 approval from maintainers, all CI checks must pass
- **Release tags (`v*`)**: Only maintainers can create release tags

**You cannot:**
- Push directly to `main`
- Create release tags
- Merge your own PRs

**All changes must go through Pull Requests.**

## What We're Looking For

### High Priority Contributions

- Bug fixes with test coverage
- Performance improvements with benchmarks
- Documentation improvements
- Integration tests for edge cases

### Medium Priority

- New features discussed in Issues first
- Refactoring with clear benefits
- Additional CDC event handlers

### Please Discuss First

Before working on these, please open an Issue for discussion:

- Major architectural changes
- New external dependencies
- Breaking API changes
- Security-related features

## Code Review Standards

Maintainers will review PRs for:

- **Correctness**: Does it work as intended?
- **Tests**: Are there adequate tests?
- **Simplicity**: Is this the simplest solution?
- **Security**: Are there any security implications?
- **Performance**: Does this impact performance?

## CI/CD Pipeline

Our CI runs automatically on all PRs:

### Pull Request Checks
```yaml
- Go tests (all packages)
- go vet (static analysis)
- gofmt (code formatting)
- Build verification
```

### Release Process (Maintainers Only)

Releases are triggered by pushing tags:

```bash
# Only maintainers can do this
git tag v0.2.0
git push origin v0.2.0
```

This triggers:
1. Run all tests
2. Build binaries for all platforms
3. Create GitHub Release
4. Build and push Docker images

## Getting Help

- **Questions**: Open a [Discussion](https://github.com/witnz/witnz/discussions)
- **Bugs**: Open an [Issue](https://github.com/witnz/witnz/issues) with reproduction steps
- **Features**: Open an [Issue](https://github.com/witnz/witnz/issues) to discuss first

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see [LICENSE](LICENSE)).

## Recognition

Contributors will be:
- Listed in release notes
- Credited in the repository
- Mentioned in project documentation

Thank you for contributing to Witnz! 🎉
