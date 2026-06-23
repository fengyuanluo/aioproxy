# ADR 0062: Main push publishes a continuous release

## Context

AIOPROXY previously published formal GitHub Releases only from `v*` tags. Main branch pushes still built every platform package, but they deliberately avoided Actions artifacts after repository-owner artifact quota failures. The project now needs a public-repository workflow where every accepted `main` push can produce downloadable binaries without relying on Actions artifact storage.

## Decision

AIOPROXY keeps versioned releases on explicit `v*` tags and adds an automatic continuous release for `main` pushes.

On every `push` to `main`, the release job:

1. waits for tests and the platform build matrix to pass;
2. force-moves the `continuous` tag to the pushed commit;
3. rebuilds Linux, macOS, and Windows amd64/arm64 packages inside the release job;
4. uploads packages, per-file SHA256 files, and `checksums.txt` to the GitHub Release attached to `continuous`;
5. marks the continuous release as a prerelease and does not replace the latest stable version.

On every `v*` tag push, the release job still publishes the versioned release for that tag and marks it as the latest stable release.

Pull requests run validation only and never publish releases.

## Consequences

- Public users can download the newest main-branch binaries from the `continuous` release without waiting for a version tag.
- The workflow still avoids Actions artifact storage as a release handoff mechanism.
- `v*` tags remain the stable distribution channel.
- The `continuous` tag is intentionally mutable and must not be used as an immutable version reference.
- The release job requires `contents: write`; other jobs keep read-only repository permissions.
