# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Default value shown for `--logging.main.level`.

## [0.5.0] - 2023-11-15

### Changed

- Functions returned from `WithContext` can be `nil` when used in `NewHandler`.

## [0.4.0] - 2023-11-03

## Added

- `NewHandler` to be used as a middleware with `WithContext` to add logger to the context.

## Fixed

- Durations are logged only with millisecond precision.

## [0.3.0] - 2023-10-19

## Fixed

- Only set fields which are provided in logging config.

## [0.2.0] - 2023-10-18

### Changed

- Change `--logging.context.conditional-level` and `--logging.context.trigger-level`
  flags to `--logging.context.conditional` and `--logging.context.trigger`,
  respectively.

## [0.1.0] - 2023-10-17

### Added

- First public release.

[unreleased]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.5.0...main
[0.5.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.4.0...v0.5.0
[0.4.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.3.0...v0.4.0
[0.3.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.2.0...v0.3.0
[0.2.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.1.0...v0.2.0
[0.1.0]: https://gitlab.com/tozd/go/zerolog/-/tags/v0.1.0

<!-- markdownlint-disable-file MD024 -->
