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
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.6/SApp.xcframework.zip",
            checksum: "a85efeb84d76d9e4adfa049b12dd320da9afde61cc2482677813bf97923417cd"
        ),
        .binaryTarget(
            name: "SAppUI",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.6/SAppUI.xcframework.zip",
            checksum: "cefaa14c452c7e0b65fcf4ae0f810b523aa7ed998fec4967068164ef8f04b199"
        ),
    ]
)
