# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
except that the first subsection is called `New` rather than `Added`. The release
workflow matches on `### New`; renaming it silently drops the section from the
release notes.

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### New
- `unspammer -version` prints the version.
- A man page, `unspammer(8)`, ships with every release archive. It documents the options, every configuration key and the `<task>.cnt` counter file.

### Changed
- Releases now carry archives for Linux (amd64, arm64), macOS (Intel, Apple Silicon), Windows (amd64, arm64) and illumos, each with the README, the licence and the man page. Release tags now start with `v`, for example `v0.5.0`.
- unspammer installs from `.deb` and `.rpm` packages in the package registry at gitea.oetiker.ch, and on macOS and Linux with Homebrew (`brew install oposs/unspammer/unspammer`).

### Fixed
- A message that a task stored with `storeCopyIn` was handled again on every scan, and the log showed `cannot send literal: no continuation request received`. The copy is now stored after the folder scan has finished, so each message is handled once. (#5)

## 0.4.0 - 2023-01-09
### Changed
- The task key `forwardCopyTo` is now `forwardCopiesTo` and takes a list of addresses.
