---
title: "Safari Extension"
sidebar_label: "Safari Extension"
sidebar_position: 5
---

# Safari Extension

The joe-links Safari extension intercepts bare-hostname navigation (e.g. `go/slack`) and redirects it to your joe-links server, matching the behavior of the Chrome and Firefox extensions. Because Safari only accepts extensions through the App Store, distributing it requires an Apple Developer account and a short Xcode signing step.

## Prerequisites

| Requirement | Notes |
|-------------|-------|
| macOS 13 (Ventura) or later | Required by Xcode 15+ |
| Xcode 15+ | Free from the Mac App Store |
| Apple Developer account | $99/year at [developer.apple.com](https://developer.apple.com/programs/enroll/) |
| joe-links source code | `git clone https://github.com/joestump/joe-links.git` |

:::tip Personal use without a developer account
If you only need the extension on your own Mac and don't want to pay for a developer account, you can run Safari in [Developer mode](#developer-mode-no-account-required) and load the extension unsigned. It works fine for personal homelab use.
:::

---

## Developer Mode (no account required)

If you just want the extension on your own machine:

1. **Enable Safari Developer menu**: Safari → Settings → Advanced → check **Show features for web developers**.

2. **Allow unsigned extensions**: Develop menu → **Allow Unsigned Extensions** (you'll need to re-enable this after each macOS update).

3. **Clone the repo**:
   ```bash
   git clone https://github.com/joestump/joe-links.git
   cd joe-links
   ```

4. **Build and run in Xcode**:
   - Open `integrations/apple/joe-links.xcodeproj`
   - Select the **joe-links (macOS)** scheme
   - Press **⌘R** to build and run — this installs the app bundle containing the extension

5. **Enable in Safari**: Safari → Settings → Extensions → check **joe-links**. Grant the requested permissions.

6. **Configure the extension**: Click the joe-links toolbar icon → open the extension settings → set your server URL (e.g. `http://go`) and paste in your API key from Dashboard → Settings → API Tokens.

---

## Signing for personal distribution (Ad-hoc / TestFlight)

If you want to share the extension with a few people (e.g. your household or team) without publishing to the App Store, you can distribute an ad-hoc signed build.

### 1. Configure signing in Xcode

1. Open `integrations/apple/joe-links.xcodeproj`
2. Select the project root → **Signing & Capabilities** tab
3. Under **Team**, choose your Apple Developer account
4. Set **Bundle Identifier** to something unique, e.g. `com.yourname.joe-links`
5. Make sure both the app target (`joe-links (macOS)`) and the extension target (`joe-links Extension (macOS)`) have the same team selected

### 2. Build an archive

```bash
cd integrations/apple/joe-links
xcodebuild archive \
  -scheme "joe-links (macOS)" \
  -archivePath build/joe-links.xcarchive \
  CODE_SIGN_IDENTITY="Developer ID Application" \
  CODE_SIGN_STYLE=Manual \
  DEVELOPMENT_TEAM=YOUR_TEAM_ID
```

Or use Xcode's GUI: Product → **Archive**.

### 3. Export and notarize

In Xcode Organizer (Window → Organizer → Archives):

1. Select the archive → **Distribute App**
2. Choose **Developer ID** distribution (for direct download outside the App Store)
3. Follow the prompts — Xcode submits the binary to Apple's notary service automatically
4. Once approved (usually a few minutes), you get a signed `.dmg` or `.zip`

Recipients double-click the app to install, then enable the extension in Safari → Settings → Extensions.

---

## App Store distribution

Publishing to the Mac App Store lets anyone install the extension through Safari's built-in Extensions Gallery.

### 1. Prepare App Store Connect

1. Sign in at [appstoreconnect.apple.com](https://appstoreconnect.apple.com)
2. Click **+** → **New App** → macOS
3. Fill in the app name, bundle ID (`com.yourname.joe-links`), SKU, and primary language
4. Set a price (free is fine)

### 2. Create App Store screenshots

Apple requires at least one screenshot for each supported Mac display size. You can capture them from the Simulator or a real Mac:

- At minimum: one 1280×800 screenshot showing the extension popup or options page
- Recommended: also include a 1440×900 screenshot

### 3. Build and upload

```bash
xcodebuild archive \
  -scheme "joe-links (macOS)" \
  -archivePath build/joe-links.xcarchive

xcodebuild -exportArchive \
  -archivePath build/joe-links.xcarchive \
  -exportPath build/export \
  -exportOptionsPlist ExportOptions.plist
```

Or archive via Xcode (Product → Archive) then in Organizer → **Distribute App** → **App Store Connect** → **Upload**.

### 4. Submit for review

Back in App Store Connect:
1. Go to your app's page → **TestFlight** tab to test the build first (optional but recommended)
2. Switch to the **App Store** tab → create a new version → attach the uploaded build
3. Fill in the description, keywords, and support URL
4. Click **Submit for Review**

Apple typically reviews Safari extensions within 1–3 business days.

---

## Automating with CI (GitHub Actions)

You can automate the archive and notarization step in GitHub Actions on tag pushes. This requires storing your Apple credentials as repository secrets.

### Required secrets

| Secret | Value |
|--------|-------|
| `APPLE_CERTIFICATE_BASE64` | Base64-encoded `.p12` exported from Keychain |
| `APPLE_CERTIFICATE_PASSWORD` | Password for the `.p12` |
| `APPLE_TEAM_ID` | Your 10-character Team ID from developer.apple.com |
| `APPLE_ID` | Your Apple ID email |
| `APPLE_APP_PASSWORD` | App-specific password from [appleid.apple.com](https://appleid.apple.com) |

Export your Developer ID certificate from Keychain Access:

```bash
# After exporting MyDeveloperID.p12 from Keychain Access:
base64 -i MyDeveloperID.p12 | pbcopy   # copies to clipboard → paste into GitHub secret
```

### Add a `safari` job to ci.yml

```yaml
safari:
  name: Safari Extension
  needs: [lint, test]
  if: startsWith(github.ref, 'refs/tags/')
  runs-on: macos-latest
  steps:
    - uses: actions/checkout@v4

    - name: Import signing certificate
      env:
        CERTIFICATE_BASE64: ${{ secrets.APPLE_CERTIFICATE_BASE64 }}
        CERTIFICATE_PASSWORD: ${{ secrets.APPLE_CERTIFICATE_PASSWORD }}
      run: |
        KEYCHAIN_PATH=$RUNNER_TEMP/build.keychain
        security create-keychain -p "" $KEYCHAIN_PATH
        security set-keychain-settings -lut 21600 $KEYCHAIN_PATH
        security unlock-keychain -p "" $KEYCHAIN_PATH
        echo "$CERTIFICATE_BASE64" | base64 --decode > $RUNNER_TEMP/cert.p12
        security import $RUNNER_TEMP/cert.p12 -P "$CERTIFICATE_PASSWORD" \
          -k $KEYCHAIN_PATH -T /usr/bin/codesign
        security list-keychain -d user -s $KEYCHAIN_PATH

    - name: Build Safari extension
      run: |
        cd integrations/apple
        xcodebuild archive \
          -scheme "joe-links (macOS)" \
          -archivePath $RUNNER_TEMP/joe-links.xcarchive \
          CODE_SIGN_IDENTITY="Developer ID Application" \
          DEVELOPMENT_TEAM=${{ secrets.APPLE_TEAM_ID }}

    - name: Notarize and export
      env:
        APPLE_ID: ${{ secrets.APPLE_ID }}
        APPLE_APP_PASSWORD: ${{ secrets.APPLE_APP_PASSWORD }}
        APPLE_TEAM_ID: ${{ secrets.APPLE_TEAM_ID }}
      run: |
        xcodebuild -exportArchive \
          -archivePath $RUNNER_TEMP/joe-links.xcarchive \
          -exportPath dist/safari \
          -exportOptionsPlist integrations/apple/ExportOptions.plist
        # Notarize
        xcrun notarytool submit dist/safari/joe-links.dmg \
          --apple-id "$APPLE_ID" \
          --password "$APPLE_APP_PASSWORD" \
          --team-id "$APPLE_TEAM_ID" \
          --wait
        xcrun stapler staple dist/safari/joe-links.dmg

    - name: Upload to release
      uses: softprops/action-gh-release@v2
      with:
        files: dist/safari/joe-links.dmg
```

:::note
The `ExportOptions.plist` file controls distribution method (Developer ID vs App Store). Create it once in Xcode during a manual export — Xcode saves it to the export folder — then commit it to the repo.
:::

---

## Updating the extension

When the extension source changes (new features, bug fixes):

1. Pull the latest code:
   ```bash
   git pull origin main
   ```
2. Open `integrations/apple/joe-links.xcodeproj` in Xcode
3. Bump the version number: select the app target → **General** → **Version**
4. Press **⌘R** to build and run (for local testing), or archive and distribute as above
5. For App Store builds: create a new version in App Store Connect, attach the new build, re-submit

The Xcode project is maintained in the repository — no conversion step is needed. All extension logic lives in `integrations/extension/`; the Xcode project references those files directly.
