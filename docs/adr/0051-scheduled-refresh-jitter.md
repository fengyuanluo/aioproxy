# ADR 0051: Apply jitter to scheduled refreshes

## Status
Accepted

## Context
AIOPROXY v1 refreshes active plugins immediately on startup and then continues refreshing them on plugin-level schedules. Different plugins can have different refresh intervals, and failed refreshes place the plugin into degraded state while future scheduled refreshes continue.

A product boundary was needed for whether refresh schedules must run at exact configured times or may include randomized offset to avoid synchronized request spikes against FOFA, subscription URLs, FPL, or other upstream sources.

## Decision
AIOPROXY v1 applies jitter to scheduled plugin refreshes by default.

The startup refresh remains immediate and does not wait for jitter. Subsequent scheduled refreshes may occur with a small randomized offset around the configured interval. The jitter behavior exists for operational stability rather than as a user-facing scheduling strategy.

The jitter range is configurable or has a documented default. Documentation must state that actual scheduled refresh times can slightly vary around the configured interval.

## Consequences
- Multiple plugins and sources are less likely to refresh at the exact same instant.
- Upstream API and subscription-source request spikes are reduced.
- Logs will not show perfectly fixed refresh timestamps after startup.
- Operators should treat configured refresh intervals as approximate scheduled periods rather than exact wall-clock guarantees.
