// The Compose UI kit's example app, built against the published sApp artifacts.
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositories {
        google()
        mavenCentral()
        maven { url = uri("https://goodneo-solutions.github.io/sapp-releases/maven") }
    }
}

rootProject.name = "sapp-example-android"
include(":app")
