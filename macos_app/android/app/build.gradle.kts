import java.io.File
import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.example.winminer_app"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "com.example.winminer_app"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
        ndk {
            abiFilters += "arm64-v8a"
        }
    }

    buildTypes {
        release {
            // TODO: Add your own signing config for the release build.
            // Signing with the debug keys for now, so `flutter run --release` works.
            signingConfig = signingConfigs.getByName("debug")
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}

val goSourceRoot = rootProject.projectDir.parentFile.parentFile
val goLibrary = layout.buildDirectory.file("generated/go/arm64-v8a/libwinminer.so")
val goJniLibsDirectory = layout.buildDirectory.dir("generated/winminerJniLibs")
val goNdkVersion = flutter.ndkVersion
val localProperties = Properties().apply {
    val propertiesFile = rootProject.file("local.properties")
    if (propertiesFile.exists()) {
        propertiesFile.inputStream().use { load(it) }
    }
}
val androidSdkDirectory = localProperties.getProperty("sdk.dir")?.let { File(it) }
    ?: System.getenv("ANDROID_SDK_ROOT")?.let { File(it) }
    ?: System.getenv("ANDROID_HOME")?.let { File(it) }

android.sourceSets.getByName("main").jniLibs.srcDir(goJniLibsDirectory.get().asFile)

val buildGoLibrary by tasks.registering(Exec::class) {
    inputs.files(
        fileTree(goSourceRoot) {
            include("*.go", "*.toml", "go.mod", "go.sum")
        },
    )
    outputs.file(goLibrary)
    workingDir(goSourceRoot)

    doFirst {
        val sdkDirectory = androidSdkDirectory
            ?: throw GradleException(
                "Android SDK not found. Configure flutter doctor before building.",
            )
        val ndkDirectory = sdkDirectory.resolve("ndk/$goNdkVersion")
        val clang = sequenceOf("darwin-arm64", "darwin-x86_64")
            .map { ndkDirectory.resolve("toolchains/llvm/prebuilt/$it/bin/aarch64-linux-android21-clang") }
            .firstOrNull { it.isFile }
            ?: throw GradleException("Android NDK compiler not found in $ndkDirectory")
        goLibrary.get().asFile.parentFile.mkdirs()
        environment("GOOS", "android")
        environment("GOARCH", "arm64")
        environment("CGO_ENABLED", "1")
        environment("CC", clang.absolutePath)
        commandLine(
            "go",
            "build",
            "-buildmode=c-shared",
            "-o",
            goLibrary.get().asFile.absolutePath,
            ".",
        )
    }
}

val stageGoLibrary by tasks.registering(Copy::class) {
    dependsOn(buildGoLibrary)
    from(goLibrary)
    into(goJniLibsDirectory.map { it.dir("arm64-v8a") })
}

tasks.named("preBuild").configure {
    dependsOn(stageGoLibrary)
}
