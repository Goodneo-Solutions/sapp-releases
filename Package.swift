// swift-tools-version: 5.9
// sApp for iOS and macOS: SApp (the core) and SAppUI (the SwiftUI kit), as prebuilt XCFrameworks.
// Documentation: https://open.superroute.ca/developer/sapp
import PackageDescription

let package = Package(
    name: "SApp",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [
        .library(name: "SApp", targets: ["SApp"]),
        // The kit is built over the core, so its product carries both.
        .library(name: "SAppUI", targets: ["SAppUI", "SApp"]),
    ],
    targets: [
        .binaryTarget(
            name: "SApp",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.5/SApp.xcframework.zip",
            checksum: "ec017643b7fe91aaeb25ac8e3a3136563a1c0df016f2692b54b81e0acc2374f4"
        ),
        .binaryTarget(
            name: "SAppUI",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.5/SAppUI.xcframework.zip",
            checksum: "641ac29a7feed01771d92c15d86a4e56a1a6801f4d2ae9e5d423a95891754c8e"
        ),
    ]
)
