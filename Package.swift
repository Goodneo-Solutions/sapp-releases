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
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.1/SApp.xcframework.zip",
            checksum: "fffef925420eafa20870eff2f7dbe8f7e3ed063b01276965cd10729b603e9a30"
        ),
        .binaryTarget(
            name: "SAppUI",
            url: "https://github.com/Goodneo-Solutions/sapp-releases/releases/download/0.2.1/SAppUI.xcframework.zip",
            checksum: "e23786156fceff9996ee0e683ebaf4b5d465c832ce375dcc399ac3d107335a3b"
        ),
    ]
)
