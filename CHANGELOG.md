# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.3] - 2021-11-23
### Fixed
- `Expiration time' claim ('exp') is too far in the future` by allowing 1 minute time drift for expiration (JWT is now valid for 9 minutes only)

## [0.0.2] - 2021-11-18
### Fixed
- NPE on automatic installation ID lookup based on `--installation` path in CLI mode

## [0.0.1] - 2021-11-16
### Added
- Initial release
