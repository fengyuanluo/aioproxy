# ADR 0058: Build artifacts on main push and publish releases on tags

## Status
Accepted

## Context
AIOPROXY v1 is released as multi-platform binaries through GitHub Actions. The project does not publish Docker images in v1, uses stage-based Git commits, and needs both continuous build evidence and a clear formal release path.

A product boundary was needed for whether every main push should create downloadable artifacts, whether only tags should build release assets, or whether releases should be triggered manually.

## Decision
AIOPROXY v1 uses a hybrid GitHub Actions release model.

Pushes to the main branch build and upload multi-platform binary artifacts as CI artifacts, but do not create GitHub Releases.

Version tags trigger the formal release workflow. Tag-triggered releases build multi-platform binaries, package archives, generate checksums, create a GitHub Release, and upload release assets.

Pull requests run validation such as tests and builds for review, but do not create formal releases.

## Consequences
- Main branch commits produce downloadable CI artifacts for testing and handoff without polluting GitHub Releases.
- Formal releases remain tied to explicit version tags.
- Users can distinguish temporary CI artifacts from versioned release assets.
- Release documentation must explain that CI artifacts are not stable releases and that tagged releases are the official distribution channel.
- Workflow validation must cover both the continuous build path and the tag release path.
