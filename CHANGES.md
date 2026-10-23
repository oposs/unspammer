# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
except that the first subsection is called `New` rather than `Added`. The release
workflow matches on `### New`; renaming it silently drops the section from the
release notes.

This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### New

### Changed

### Fixed

## 0.5.0 - 2026-10-23
### New
- `unspammer -version` prints the version.
- A man page, `unspammer(8)`, ships with every release archive. It documents the options, every configuration key and the `<task>.cnt` counter file.

### Changed
- Releases now carry archives for Linux (amd64, arm64), macOS (Intel, Apple Silicon), Windows (amd64, arm64) and illumos, each with the README, the licence and the man page. Release tags now start with `v`, for example `v0.5.0`.
- unspammer installs from `.deb` and `.rpm` packages in the package registry at gitea.oetiker.ch, and on macOS and Linux with Homebrew (`brew install oposs/unspammer/unspammer`).

### Fixed
- unspammer crashed with `panic: runtime error: invalid memory address or nil pointer dereference` when it could not reach an IMAP server, instead of retrying after 15 seconds.
- A task with `editCopy: rt-tag` no longer stops the start when its `<task>.cnt` file is missing; the count starts at 1. A counter file without a `-`, for example `26`, crashed unspammer with `index out of range`; it now stops the start with a message that names the file.
- `-jsonnetdump` now prints the JSON to standard output, as its help text says, instead of to the service log.
- A message that a task stored with `storeCopyIn` was handled again on every scan, and the log showed `cannot send literal: no continuation request received`. The copy is now stored after the folder scan has finished, so each message is handled once. (#5)

## 0.4.0 - 2023-01-09
### Changed
- The task key `forwardCopyTo` is now `forwardCopiesTo` and takes a list of addresses.
