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
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.0/SApp.xcframework.zip",
            checksum: "b805b6275c8cf5689bd86e5df86c5a14b60db672b861fd3c160e78a830f5266b"
        ),
        .binaryTarget(
            name: "SAppUI",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.0/SAppUI.xcframework.zip",
            checksum: "a2a0985ed765d9bb911b27aeec9f60af94391f82a954300080b7ad772a6f3f1d"
        ),
    ]
)
