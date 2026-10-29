<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/Goodneo-Solutions/sapp-releases/main/assets/sapp-logo-dark.svg">
  <img src="https://raw.githubusercontent.com/Goodneo-Solutions/sapp-releases/main/assets/sapp-logo.svg" alt="sApp" height="56">
</picture>

# sApp

Build apps on Superroute: customer and driver apps that run for one or more service providers, and apps that use
Open Platform capabilities such as routing, geocoding and ETA. MIT licence.

Documentation: [open.superroute.ca/developer/sapp](https://open.superroute.ca/developer/sapp). Swift packages, the
Maven repository and the example apps: [Goodneo-Solutions/sapp-releases](https://github.com/Goodneo-Solutions/sapp-releases).

| Package | Platform | Status |
|---|---|---|
| [`@superrouteca/sapp`](https://www.npmjs.com/package/@superrouteca/sapp) | JavaScript and TypeScript: web, Node 20+, React Native | 0.2.0 |
| [`@superrouteca/sapp-react`](https://www.npmjs.com/package/@superrouteca/sapp-react) | React: provider and hooks | 0.2.0 |
| [`@superrouteca/sapp-react-native`](https://www.npmjs.com/package/@superrouteca/sapp-react-native) | React Native: hooks, secure token store | 0.2.0 |
| [`sapp`](https://pub.dev/packages/sapp) | Dart and Flutter | 0.2.0 |
| [`SApp`](https://github.com/Goodneo-Solutions/sapp-releases) (Swift Package Manager) | iOS 15+, macOS 12+ | 0.2.0 |
| [`ca.superroute:sapp` / `sapp-android`](https://github.com/Goodneo-Solutions/sapp-releases) (Maven) | Kotlin on the JVM and Android 7+ | 0.2.0 |
| [`Superroute.SApp`](https://www.nuget.org/packages/Superroute.SApp) (NuGet) | .NET 8: Windows apps and servers | 0.2.0 |
| [`sapp_ui`](https://pub.dev/packages/sapp_ui) | Flutter UI kit: sign-in, notifications, wallet, orders, pickups | 0.2.0 |
| [`@superrouteca/sapp-ui`](https://www.npmjs.com/package/@superrouteca/sapp-ui) | Web UI kit: the same components as custom elements | 0.2.0 |
| `@superrouteca/sapp-react/ui` | React components over the web UI kit | 0.2.0 |
| [`SAppUI`](https://github.com/Goodneo-Solutions/sapp-releases) (Swift Package Manager) | SwiftUI UI kit: iOS 15+, macOS 12+ | 0.2.0 |
| [`ca.superroute:sapp-ui-android`](https://github.com/Goodneo-Solutions/sapp-releases) (Maven) | Jetpack Compose UI kit: Android 7+ | 0.2.0 |
| `@superrouteca/sapp-react-native/ui` | React Native UI kit | 0.2.0 |
| [`Superroute.SApp.Wpf`](https://www.nuget.org/packages/Superroute.SApp.Wpf) / [`Superroute.SApp.WinForms`](https://www.nuget.org/packages/Superroute.SApp.WinForms) (NuGet) | WPF and WinForms UI kits over `Superroute.SApp.UI` | 0.2.0 |

Every package of a release carries the same version. Version numbers never contain the digits 3 or 4.

## Profiles

- **Partner** — an app registered on the Open Platform console (`open.superroute.ca/open-api/apps`). It acts for one
  person at one service provider, with that person's sign-in and the capabilities the app was approved for and the
  provider switched on.
- **Open** — Open Platform capabilities (`open.superroute.ca/v1`). Inside an app, pass `getToken`, answered by your
  own backend with a short-lived token; an Open Platform API key is a server secret and never belongs in an app.

What a caller may do is decided by the server, never by the package installed.

## JavaScript

```js
import { createPartnerClient } from '@superrouteca/sapp';

const sr = createPartnerClient({
  appCode: 'p_acme_parcels',
  clientCode: 'k3m9q2',          // the service provider's public client code
  appKey: 'sp_appkey_…',         // restricted key for calls before sign-in
  tokenStore,                    // your secure storage: { get, set, clear }
  locale: 'en',
});

// Sign in with Google or Apple in the app…
await sr.auth.signInWithIdToken(googleIdToken);
// …or on the Superroute sign-in page:
const { url, verifier } = await sr.auth.authorizationUrl({ redirectUri: 'com.acme.parcels:/oauth' });
// open `url` in the system browser, then
await sr.auth.exchangeCode({ code, redirectUri: 'com.acme.parcels:/oauth', verifier });

const notifications = await sr.api.platformAppNotifications();
const me = await sr.graphql('{ partnerAppMe { app_code user_name } }');
```

## React and React Native

```jsx
import { SuperrouteProvider, useApi, useSession, createSecureTokenStore } from '@superrouteca/sapp-react-native';

const sr = createPartnerClient({ appCode: 'p_acme_parcels', clientCode, appKey, tokenStore: createSecureTokenStore(secureStorage) });

function Inbox() {
  const { signedIn } = useSession();
  const { data, loading, error, reload } = useApi((api) => api.platformAppNotifications(), [], { enabled: signedIn === true });
  …
}

<SuperrouteProvider client={sr}><Inbox /></SuperrouteProvider>
```

Sign-in on the Superroute page from a phone: `sr.auth.signInWithBrowser({ redirectUri, open })`, where `open(url,
redirectUri)` shows the page in an in-app browser session and answers the URL it returned to (null when closed).
`sr.auth.subscribe(fn)` is told when a session starts and ends.

## Dart and Flutter

```dart
import 'package:sapp/sapp.dart';

final sr = PartnerClient(appCode: 'p_acme_parcels', clientCode: 'k3m9q2', appKey: 'sp_appkey_…', tokenStore: store);
await sr.signInWithIdToken(googleIdToken);
final notifications = await sr.api.platformAppNotifications();
```

## Swift

```swift
// Package.swift
.package(url: "https://github.com/Goodneo-Solutions/sapp-releases", from: "0.2.0")
// in the target: .product(name: "SApp", package: "sapp-releases"), and .product(name: "SAppUI", package: "sapp-releases") for the UI kit
```

In Xcode: File > Add Package Dependencies…, `https://github.com/Goodneo-Solutions/sapp-releases`.

```swift
import SApp

let sr = PartnerClient(appCode: "p_acme_parcels", clientCode: "k3m9q2", appKey: "sp_appkey_…", tokenStore: KeychainTokenStore())
try await sr.signIn(idToken: googleIdToken)
let notifications = try await sr.api.platformAppNotifications()

// The Superroute sign-in page in an ASWebAuthenticationSession:
let request = try await sr.authorizationRequest(redirectURI: "com.acme.parcels:/oauth")
// … present request.url, then with the URL the session returned:
try await sr.completeSignIn(request, returnedURL: callbackURL)
```

Answers are `JSON` values (`answer["data"]?["items"]?.array`). `onSessionChange` is told when a session starts and ends.

## Kotlin and Android

```kotlin
// settings.gradle.kts
dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://goodneo-solutions.github.io/sapp-releases/maven") }
    }
}

// build.gradle.kts: ca.superroute:sapp on the JVM, sapp-android on Android, sapp-ui-android for the Compose kit
implementation("ca.superroute:sapp-android:0.2.0")
implementation("ca.superroute:sapp-ui-android:0.2.0")
```

```kotlin

val sr = PartnerClient(appCode = "p_acme_parcels", clientCode = "k3m9q2", appKey = "sp_appkey_…", tokenStore = KeystoreTokenStore(context))
sr.signInWithIdToken(googleIdToken)
val notifications = sr.api.platformAppNotifications()   // a kotlinx.serialization JsonElement
```

The sign-in page: open `sr.authorizationRequest(redirectUri).url` in a Custom Tab, then `sr.completeSignIn(request,
returnedUrl)`. `KeystoreTokenStore` encrypts the tokens with a key held in the Android Keystore.

## C# and .NET

```csharp
using Superroute.SApp;

using var sr = new PartnerClient("p_acme_parcels", clientCode: "k3m9q2", appKey: "sp_appkey_…", tokenStore: new ProtectedDataTokenStore());
await sr.SignInWithIdTokenAsync(googleIdToken);
var notifications = await sr.Api.PlatformAppNotificationsAsync();   // a System.Text.Json JsonNode
```

`ProtectedDataTokenStore` keeps the tokens encrypted for the current Windows user. `OnSessionChange` is told when a
session starts and ends.

## Flutter UI kit

```dart
SuperrouteScope(
  client: sr,
  theme: ThemeTokens.fromBranding(bootstrap['client']['settings']['branding']),
  onSignInRequired: () => router.go('/sign-in'),
  child: SrPickupList(onShowCode: (value) => showQr(value)),
)
```

Components, for customer apps: `SrSignIn`, `SrNotificationInbox`, `SrWalletSummary`, `SrOrderList`, `SrPickupList`,
`SrOrderDetail`, `SrTracking`, `SrNotificationPreferences`, `SrAccountPrivacy`, `SrLockerMap` (with
`SrSMapAdapter(controller)` for the app's sMap map), `SrReturnList`, `SrReturnForm`, `SrShippingOrderForm`, the store
and mall (`SrStoreDirectory`, `SrStorePage`, `SrProductDetail`, `SrCart`, `SrCheckout`) and group buy
(`SrGroupBuyList`, `SrGroupBuy`, `SrGroupBuyOrders`, `SrGroupBuyOrder`, and for group leaders `SrGroupBuyLead`,
`SrGroupBuyLeading`, `SrGroupBuyLeadingGroup`); for driver apps: `SrDriverRoutes`,
`SrRouteStops`, `SrStop` (photos, a signature pad, the delivery PIN), `SrDriverDuty`, `SrDriverPool`, `SrRouteBoards`,
`SrRouteBoard`, `SrBoardClaims`, `SrDriverEarnings`, `SrScanLookup`. Files, the position, the scanner and card payments
are the app's: a component takes a callback for them (`pickFile`, `pickPhoto`, `getPosition`, `scan`) or hands the
payment or a link to share to the app (`onPayment`, `onShare`). They raise events and
never navigate; what each shows, its states and strings are in the [UI kit reference](https://open.superroute.ca/developer/sapp), the same on every
platform. Strings in 14 languages live in `ui/strings/` (`tools/ui_strings.py`).

## Web and React UI kit

```html
<script type="module">
  import { configure } from '@superrouteca/sapp-ui';
  configure({ client: sr, theme: themeFromBranding(branding), locale: 'en' });
</script>
<sr-pickup-list></sr-pickup-list>
```

Elements: `sr-sign-in`, `sr-notification-inbox`, `sr-wallet-summary`, `sr-order-list`, `sr-pickup-list`,
`sr-order-detail`, `sr-tracking`, `sr-notification-preferences`, `sr-account-privacy`, `sr-locker-map` (`map =
smapAdapter(sMapMap)`), `sr-return-list`, `sr-return-form`, `sr-shipping-order-form`, `sr-store-directory`,
`sr-store-page`, `sr-product-detail`, `sr-cart`, `sr-checkout`, `sr-group-buy-list`, `sr-group-buy`,
`sr-group-buy-orders`, `sr-group-buy-order`, for group leaders `sr-group-buy-lead`, `sr-group-buy-leading`,
`sr-group-buy-leading-group`, and for driver apps `sr-driver-routes`, `sr-route-stops`, `sr-stop`,
`sr-driver-duty`, `sr-driver-pool`, `sr-route-boards`, `sr-route-board`, `sr-board-claims`, `sr-driver-earnings`,
`sr-scan-lookup`. Each raises `sr-*` events (`sr-open`, `sr-created`, `sr-payment`, `sr-completed`, …, listed in the
types) and `sr-sign-in-required`. The kit never draws a map: the locker map uses the app's own sMap map through the adapter. In React:

```jsx
import { PickupList } from '@superrouteca/sapp-react/ui';
<SuperrouteProvider client={sr}><PickupList onShowCode={showQr} /></SuperrouteProvider>
```

## Native UI kits

The same components on SwiftUI, Jetpack Compose, React Native, WPF and WinForms, following the [UI kit reference](https://open.superroute.ca/developer/sapp)
exactly as the Flutter and web kits do, with the same strings in 14 languages (generated from `ui/strings/` by
`tools/ui_strings.py`). Each component keeps its state and calls in a testable model (an `ObservableObject` on SwiftUI,
a state holder on Compose, a view model in `Superroute.SApp.UI` shared by WPF and WinForms) and draws it with the
platform's own controls. The app supplies what the kit leaves to it: images on Compose, the in-app browser for sign-in,
files, photos, the position, the scanner, card payments and the sMap map through its adapter.

```swift
import SAppUI
SuperrouteScope(client: client, theme: ThemeTokens(branding: branding)) {
    PickupList(onShowCode: { code in showQR(code) })
}
```

```kotlin
SuperrouteScope(client = client, theme = ThemeTokens(branding)) {
    PickupList(onShowCode = { code -> showQr(code) })
}
```

```jsx
import { PickupList } from '@superrouteca/sapp-react-native/ui';
<SuperrouteProvider client={sr}><PickupList onShowCode={(code) => showQr(code)} /></SuperrouteProvider>
```

Example apps, built against the published packages: [SwiftUI](https://github.com/Goodneo-Solutions/sapp-releases/tree/main/examples/swiftui), [Android](https://github.com/Goodneo-Solutions/sapp-releases/tree/main/examples/android), [React Native](https://github.com/Goodneo-Solutions/sapp-releases/tree/main/examples/react-native), [WPF](https://github.com/Goodneo-Solutions/sapp-releases/tree/main/examples/wpf) and [WinForms](https://github.com/Goodneo-Solutions/sapp-releases/tree/main/examples/winforms).

## Errors

Every failed call throws one error type (`SuperrouteError` / `SuperrouteException`) with the HTTP `status`, the
server's `code` (`invalid_token`, `insufficient_scope`, `access_denied`, `APP_ACCESS_DENIED`, …) and the parsed body.
`signInRequired` is true when the person must sign in again. Repeatable calls are retried on a lost connection and
on temporary server errors; writes are never retried.

## How the clients are made

The API methods are generated from the specs Superroute publishes (`specs/`): one method per operation. Never edit
`*/generated/*`; run `tools/sync-specs.sh /path/to/Superroute` to copy newer specs and regenerate. CI fails when the
generated files do not match the specs.

## Tests

```sh
npm install && npm test        # js, react, react-native
cd dart && dart pub get && dart test
swift test
cd kotlin && ./gradlew :sapp:test
dotnet test dotnet/tests/Superroute.SApp.Tests/Superroute.SApp.Tests.csproj
cd flutter_ui && flutter test
```
