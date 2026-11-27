# Changelog

All notable changes to Branchopus are documented in this file.

## [0.1.2] - 2026-11-27

### Added

- Hover details for every tree node, including its full name, absolute path, type, and relevant file count.

### Fixed

- Lazy-loaded child pages now reset with each search, type filter, root, or branch change.
- Stale child-page responses can no longer merge into a newer query result.

## [0.1.1] - 2026-11-26

### Added

- Context-menu actions to open any node in File Explorer and copy its absolute path.
- File nodes are selected automatically when their containing directory opens.

## [0.1.0] - 2026-11-26

### Added

- Local folder indexing and true branching-tree rendering.
- File-extension filters and name search with ancestor-path preservation.
- Safe rendering limits with complete root-to-match paths.
- One-shot expansion depth controls.
- Branch focus and “set as root” context-menu actions.
- Lightweight in-app folder picker.
- Multi-size Windows icon and Branchopus branding.
