# SPEC-0015: Safari Web Extension — Apple Distribution and CI Pipeline

## Overview

This spec defines the requirements for packaging the existing Manifest V3 Web Extension
(SPEC-0008) as a native iOS and macOS Safari Web Extension, distributing it via TestFlight,
and automating that distribution through the existing GitHub Actions CI pipeline. See ADR-0015
for the distribution strategy decision and ADR-0012 for the extension architecture decision.

---

## Requirements

### Requirement: Xcode Project Generation

The `integrations/extension/` source directory SHALL be convertible to a valid Xcode project using
`xcrun safari-web-extension-converter` without requiring modifications to the existing
JavaScript or manifest files beyond what the converter applies automatically. The generated
Xcode project MUST be committed to the repository under `integrations/apple/` and MUST target both iOS 15+
and macOS 12+ from a single project with two targets (iOS app and macOS app).

#### Scenario: Converter runs without errors

- **WHEN** a developer runs `xcrun safari-web-extension-converter integrations/extension/ --app-name "joe-links" --bundle-identifier net.joestump.joe-links --swift --macos-only false`
- **THEN** an Xcode project is created at `integrations/apple/` with no conversion errors and the existing `background.js`, `popup.html`, `popup.js`, `options.html`, `options.js`, and `manifest.json` are referenced intact

#### Scenario: Both platform targets present

- **WHEN** the generated Xcode project is opened
- **THEN** it contains an iOS app target (`joe-links (iOS)`) and a macOS app target (`joe-links (macOS)`) sharing the same extension bundle

---

### Requirement: iOS Safari Compatibility

The extension MUST function correctly in iOS 15+ Safari. Capabilities that are unavailable in
iOS Safari service workers MUST degrade gracefully without breaking extension startup or keyword
interception. Specifically, `OffscreenCanvas` (used for programmatic icon drawing) MUST NOT
cause a fatal error when unavailable; the extension MUST fall back to the PNG icons declared
in `manifest.json`.

#### Scenario: Extension loads on iOS Safari

- **WHEN** the TestFlight app is installed on an iPhone running iOS 15 or later and the extension is enabled in Settings → Safari → Extensions
- **THEN** the extension activates without errors and keyword interception is operational

#### Scenario: OffscreenCanvas unavailable

- **WHEN** the service worker attempts to call `setActionIcon()` on iOS where `OffscreenCanvas` is not supported
- **THEN** the `try/catch` in `setActionIcon()` silently swallows the error and the extension continues operating with the PNG fallback icons

#### Scenario: declarativeNetRequest rules applied on iOS

- **WHEN** the extension starts on iOS 16 or later
- **THEN** `updateRedirectRules()` successfully registers dynamic `declarativeNetRequest` rules for each keyword host; on iOS 15, the call is silently skipped if the API is unavailable

---

### Requirement: macOS Safari Compatibility

The extension MUST function correctly in macOS 12+ Safari. The macOS app wrapper MUST be
notarized using the Apple Developer Program distribution certificate so that Gatekeeper accepts
it on any Mac without requiring the user to bypass security settings.

#### Scenario: Extension installs on macOS without Gatekeeper bypass

- **WHEN** a user downloads the macOS app from TestFlight or a direct link and opens it
- **THEN** macOS Gatekeeper accepts the app without displaying a security warning, and the extension appears in Safari → Settings → Extensions

#### Scenario: Extension enables in macOS Safari

- **WHEN** the user enables the extension in Safari → Settings → Extensions and grants "Allow on all websites"
- **THEN** keyword interception is active and `go/foo` navigations redirect correctly in Safari

---

### Requirement: Code Signing Configuration

The Xcode project MUST be configured for automatic signing using an Apple Developer Program
team. Signing assets — the distribution certificate (`.p12` + passphrase) and provisioning
profiles — MUST be stored as GitHub Actions secrets and MUST NOT be committed to the repository.
The CI signing setup SHOULD use `fastlane match` or equivalent certificate management to support
reproducible signing across machines.

#### Scenario: CI builds and signs the app without local Xcode

- **WHEN** a GitHub Actions runner executes the CI release job for a tagged commit
- **THEN** the runner installs the signing certificate from GitHub Actions secrets, builds the Xcode archive, and produces a signed `.xcarchive` without requiring any interactive Xcode session

#### Scenario: Signing secrets are missing

- **WHEN** the required signing secrets (`APPLE_CERT_P12`, `APPLE_CERT_PASSWORD`, `APP_STORE_CONNECT_API_KEY`) are not present in the GitHub Actions environment
- **THEN** the CI job MUST fail with a clear error message identifying the missing secret rather than producing an unsigned or improperly signed build

---

### Requirement: TestFlight Upload

On every tagged release, the CI pipeline MUST build an `.xcarchive` for both iOS and macOS
targets and upload the resulting `.ipa` (iOS) and `.pkg` or `.app` (macOS) to TestFlight via
the App Store Connect API. The upload MUST use the App Store Connect API key (Issuer ID, Key ID,
and `.p8` private key) stored as GitHub Actions secrets. The CI job MUST fail if the upload
does not succeed.

#### Scenario: Tag triggers TestFlight upload

- **WHEN** a git tag matching `v*` is pushed and the GitHub Actions CI pipeline runs
- **THEN** the pipeline builds the Xcode archive, exports the iOS `.ipa` and macOS app, uploads both to TestFlight, and the build appears in App Store Connect under the joe-links app within 15 minutes

#### Scenario: Upload fails due to invalid API key

- **WHEN** the App Store Connect API key secret is expired or invalid
- **THEN** the CI upload step fails with a non-zero exit code and a descriptive error message; no partial build is uploaded

#### Scenario: Build number incremented on each upload

- **WHEN** a new tagged release is uploaded to TestFlight
- **THEN** the build number is derived from the git tag (e.g., `v0.2.21` → `221`) or a monotonically increasing counter, ensuring App Store Connect accepts it as a new build

---

### Requirement: TestFlight Internal Tester Access

The App Store Connect app record for joe-links MUST include at least one internal tester group.
Internal testers SHALL be added by Apple ID email in App Store Connect. TestFlight builds SHALL
be distributed to internal testers automatically upon upload without requiring additional
manual promotion steps.

#### Scenario: Developer installs extension via TestFlight

- **WHEN** an internal tester opens the TestFlight app on their iPhone after a new build is uploaded
- **THEN** the new joe-links build appears and can be installed in one tap

#### Scenario: Adding a new tester

- **WHEN** an Apple ID email is added to the internal tester group in App Store Connect
- **THEN** the user receives a TestFlight invitation email and can install the current build without waiting for a new upload

---

### Requirement: Build Expiry Management

TestFlight builds expire 90 days after upload. The CI pipeline SHOULD include a mechanism to
warn when the most recent TestFlight build is within 14 days of expiry. At minimum, the
repository MUST document the 90-day constraint and the process for uploading a refresh build.

#### Scenario: Build nearing expiry

- **WHEN** the most recent TestFlight build is 76 or more days old
- **THEN** the CI pipeline (via a scheduled workflow) SHOULD emit a warning in the GitHub Actions log and optionally open a GitHub issue titled "TestFlight build expiring soon"

#### Scenario: Build expires before refresh

- **WHEN** 90 days pass without a new upload
- **THEN** the TestFlight build is deactivated by Apple; existing installs continue to work but the build cannot be installed on new devices until a fresh upload is made

---

### Requirement: CI Job Integration

The TestFlight upload MUST be added as a new job (`safari`) in the existing `.github/workflows/ci.yml`
pipeline. The job MUST depend on the `lint` and `test` jobs (same as the existing `docker` job)
and MUST only run on tag pushes. The job MUST NOT block the existing binary release or Docker
jobs if it fails.

#### Scenario: Safari job runs in parallel with Docker job

- **WHEN** a tagged release is pushed
- **THEN** the `safari` CI job and the `docker` CI job run concurrently; a failure in `safari` does not prevent the Docker image from being built and pushed

#### Scenario: Safari job skipped on non-tag pushes

- **WHEN** a commit is pushed to `main` without a tag
- **THEN** the `safari` CI job is skipped; only `lint`, `test`, and `build` jobs run

---

### Requirement: Xcode Project Maintenance

When changes are made to the `integrations/extension/` directory that affect `manifest.json` or add new
source files, the Xcode project under `integrations/apple/` MUST be updated to reference the new files.
The CI pipeline MUST verify that the Xcode project builds successfully; a build failure in
the `safari` job MUST surface as a failed CI check on the pull request.

#### Scenario: New extension file added

- **WHEN** a new `.js` or `.html` file is added to `integrations/extension/` and referenced in `manifest.json`
- **THEN** the file is also added to the Xcode project's extension target so it is included in the Safari build

#### Scenario: Xcode build failure blocks merge

- **WHEN** a pull request introduces a change that causes the Xcode build to fail
- **THEN** the `safari` CI job fails and the pull request shows a failed check, preventing merge until the build is fixed
