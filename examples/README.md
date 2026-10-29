# sApp example apps

Example apps for the sApp UI kits, each built against the published packages of sApp 0.2.0.
Documentation: https://open.superroute.ca/developer/sapp

| Folder | Platform | Run |
|---|---|---|
| [`swiftui`](swiftui) | SwiftUI, iOS 15+ and macOS 12+ | `swift run` in the folder (macOS), or open `Package.swift` in Xcode |
| [`android`](android) | Jetpack Compose, Android 7+ | `./gradlew :app:assembleDebug` |
| [`react-native`](react-native) | React Native (Expo) | `npm install`, then `npx expo start` |
| [`wpf`](wpf) | WPF, .NET 8 on Windows | `dotnet run` |
| [`winforms`](winforms) | WinForms, .NET 8 on Windows | `dotnet run` |

No credentials are built in: enter your app code, the service provider's client code and the app key where each example says.
