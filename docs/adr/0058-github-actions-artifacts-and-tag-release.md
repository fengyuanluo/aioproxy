# ADR 0058: Avoid Actions artifacts and publish releases from trusted pushes

## Status
Accepted

## Context
AIOPROXY v1 is released as multi-platform binaries through GitHub Actions. The project does not publish Docker images in v1, uses stage-based Git commits, and needs both continuous build evidence and a clear formal release path.

A product boundary was needed for whether every main push should create downloadable artifacts, whether only tags should build release assets, or whether releases should be triggered manually.

## Decision
AIOPROXY v1 uses a hybrid GitHub Actions release model.

Pushes to the main branch validate and rebuild the multi-platform binaries, then update the mutable `continuous` prerelease. The workflow does not use Actions artifacts as a release handoff layer.

Version tags trigger the formal release workflow. Tag-triggered releases build multi-platform binaries, package archives, generate checksums, create a GitHub Release, and upload release assets.

Pull requests run validation such as tests and builds for review, but do not create formal releases.

## Consequences
- Main branch commits produce a downloadable `continuous` prerelease for testing and handoff without polluting the stable release line.
- Stable releases remain tied to explicit version tags.
- Users can distinguish the mutable `continuous` prerelease from immutable versioned release assets.
- Release documentation must explain that `continuous` is the latest main-branch build and that tagged releases are the stable distribution channel.
- Workflow validation must cover both the main-push continuous release path and the tag release path.
