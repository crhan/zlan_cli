# Releasing zlan

Releases are produced by GoReleaser and GitHub Actions. Supported artifacts are:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows builds are intentionally not published.

## Local checks

Run the same checks used by CI:

```sh
make ci
```

Build a local snapshot release into `dist/`:

```sh
make release-snapshot
```

The snapshot command does not publish anything. It verifies that all configured
archives and checksums can be produced before tagging a real release.

## Publish v0.1.0

1. Ensure `main` is clean and pushed to GitHub.
2. Create the annotated tag:

   ```sh
   make release-tag VERSION=v0.1.0
   ```

3. Push the tag:

   ```sh
   git push origin v0.1.0
   ```

GitHub Actions will run `.github/workflows/release.yml`, build the configured
platform matrix, upload archives and `checksums.txt`, and create/replace the
GitHub Release for that tag.

The version printed by `zlan version` is injected by GoReleaser through ldflags.
