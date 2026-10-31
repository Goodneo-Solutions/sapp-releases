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
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.2/SApp.xcframework.zip",
            checksum: "602c98fd9f44f08548a5ed90d62c5c2e5be4ac020dfa46ff0976e5e0b4ffe351"
        ),
        .binaryTarget(
            name: "SAppUI",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.2/SAppUI.xcframework.zip",
            checksum: "1d9fa17a9b3743f47dd49f9040f31caa349396d5dd1c259370c2cccbf8055ad3"
        ),
    ]
)
