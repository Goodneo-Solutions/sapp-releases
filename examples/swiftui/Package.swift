// swift-tools-version: 5.9
// The SwiftUI kit's example: every component against a configured client (`swift run` on macOS).
import PackageDescription

let package = Package(
    name: "SAppUIExample",
    platforms: [.iOS(.v15), .macOS(.v12)],
    dependencies: [.package(url: "https://github.com/Goodneo-Solutions/sapp-releases", exact: "0.2.5")],
    targets: [
        .executableTarget(name: "SAppUIExample", dependencies: [
            .product(name: "SApp", package: "sapp-releases"),
            .product(name: "SAppUI", package: "sapp-releases"),
        ]),
    ]
)
