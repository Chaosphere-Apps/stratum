# Contributing to Stratum

Thanks for helping improve Stratum. Changes should keep the architecture workspace reliable, accessible, and secure for self-hosted teams.

## Before you start

Read the [README](README.md) for the product and local setup, [engineering onboarding](docs/onboarding.md) for the repository map, and the relevant `backend/AGENTS.md` or `ui/AGENTS.md` before changing a service. Search existing issues and pull requests before starting large work. Discuss major API, persistence, authorization, or migration changes in an issue first.

## Make a change

1. Create a branch from the current `master` branch and keep the change focused.
2. Add or update tests for user behavior, permission boundaries, error paths, and persistence affected by the change. Use fake external services and disposable test data.
3. Keep database migrations forward-only. Never edit a migration already included in a release.
4. Update the user or operator documentation when behavior or configuration changes.
5. Keep secrets, customer data, local environment files, and temporary AI documents out of commits. Use the ignored `ai-docs/` directory for temporary AI working material only.

## Verify locally

From the repository root:

```bash
make test
make test-e2e
```

For smaller changes, run the relevant backend or UI checks in [Testing Stratum](docs/testing.md). Run `git diff --check` and review the staged diff before opening a pull request.

## Pull requests

Describe the user problem, the behavior changed, relevant security or migration effects, and the tests run. Include screenshots for visible UI changes. Pull requests into `master` require review by `@tarunwadhwa13`, the owner listed in [.github/CODEOWNERS](.github/CODEOWNERS). Repository administrators should enable GitHub's **Require review from Code Owners** branch rule for `master` to enforce this before merging.

## Report a security issue

Do not publish a vulnerability or exploit details in a public issue. Use GitHub's private vulnerability reporting for this repository when available, or contact the repository maintainers privately through the organization profile. Share a minimal reproduction and affected version without including real credentials or customer data.

## License

By contributing, you agree that your contribution is distributed under the repository's [license](LICENSE.md). Only submit work you have the right to license, and retain required third-party notices.
