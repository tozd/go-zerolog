# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Update dependencies.
- Remove parts in Kong help which we add automatically now.

## [0.8.0] - 2024-09-06

### Changed

- Go 1.23 or newer is required.

## [0.7.0] - 2024-08-18

### Changed

- Use seconds for logged durations.
- Format floating point number (including durations) to 3 digits.

### Fixed

- File logging level configuration.

## [0.6.0] - 2024-01-17

### Added

- Set main logging level using `LOGGING_MAIN_LEVEL` environment variable.

## [0.5.1] - 2023-11-17

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

[unreleased]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.8.0...main
[0.8.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.7.0...v0.8.0
[0.7.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.6.0...v0.7.0
[0.6.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.5.1...v0.6.0
[0.5.1]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.5.0...v0.5.1
[0.5.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.4.0...v0.5.0
[0.4.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.3.0...v0.4.0
[0.3.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.2.0...v0.3.0
[0.2.0]: https://gitlab.com/tozd/go/zerolog/-/compare/v0.1.0...v0.2.0
[0.1.0]: https://gitlab.com/tozd/go/zerolog/-/tags/v0.1.0

<!-- markdownlint-disable-file MD024 -->
