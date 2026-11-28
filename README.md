# winminer

A small console tool to assist winmine game in windows XP in C code. It reads the layout of field by reading pixels and analyzes the bombs location.

The original version is in winxp branch and must be complied with mingw under Windows system. Due to the changes in winmine game UI, this version ONLY works on XP.

A new version (master branch) is a rewrite in golang to focus on the AI improvements.

## Benchmark

| Level | Win | 
|-------|:--------:|
| Beginner   |    91%   |
| Intermediate |       74%     |
| Expert |   29%  |

### How to Run Benchmark

./bench.sh

It would take a few minutes.

## Rust Benchmark CLI

Build the Rust benchmark executable with `cargo build --release --bin winminer-rust`. To compare it with the Go executable using the same cases and configurations, run:

```sh
BENCH_WORKERS=1 WINMINER_BINARY=./winminer python3 bench.py
BENCH_WORKERS=1 WINMINER_BINARY=./target/release/winminer-rust python3 bench.py
```

`BENCH_WORKERS=1` runs configurations sequentially to reduce timing noise. `WINMINER_BINARY` selects the executable; it defaults to `./winminer`.

## macOS App

The Flutter macOS app embeds the Go game engine as a native library and communicates with it through a JSON C ABI. Build and run it with:

```sh
cd macos_app
flutter pub get
flutter run -d macos
```

The Xcode build phase compiles the arm64 Go library and bundles it into the app. The Flutter UI receives only the visible game snapshot; mine positions and the random seed remain inside Go.
The macOS window resizes to fit the active board, and board tiles use square corners.

## Android App

The Flutter app also supports Android arm64. Install the Android SDK and NDK, then build the APK with:

```sh
cd macos_app
flutter build apk --target-platform android-arm64
```

The Gradle build compiles the Go game engine into an `arm64-v8a` shared library and packages it in the APK.

## iOS App

The Flutter iOS app links the Go game engine as an arm64 static library. With Xcode installed, build for the iOS Simulator with:

```sh
cd macos_app
flutter build ios --simulator
```

Open `macos_app/ios/Runner.xcworkspace` in Xcode to select a simulator or configure signing for a physical device.

On iOS and Android the app is named Minesweeper. It offers 8×8/10-mine,
10×10/20-mine and 14-row × 10-column/36-mine boards, guarantees the first
reveal is safe, and shows all mines at the end. Tap to reveal, hold for 0.4
seconds to flag, or switch to flag mode.
Successful manual reveals and flag toggles play distinct sounds; hitting a mine
also plays a sound, including when a solver move triggers the loss.
The three action sounds use PCM WAV assets matched to the flag sound's RMS level.
The timer starts with the first reveal and keeps running in the background.
After five seconds without a manual move, a single-step solver becomes available
and briefly highlights the cell it selects.

## Web Game

Start the local web server:

```sh
go run . -web
```

Then open http://127.0.0.1:8080. The server uses `winminer.toml` by default; pass `-c` to select another configuration or `-addr` to change the listening address. The browser can reveal tiles manually, toggle flags with the right mouse button, or ask the bot to make one move at a time.

The HTTP API provides `GET /api/levels`, `POST /api/games`, `GET` and `DELETE /api/games/{id}`, plus `POST /api/games/{id}/click`, `/flag`, and `/bot/step`. Responses contain only revealed tiles and visible flags; the board layout and random seed stay on the server.

## Search Strategies

### Direct Search

![Direct Search](image/direct.png?raw=true)

Obviously tile A (2, 0) is safe because there is one and only one bomb around (1, 1) and (2, 1) has already been flagged there is one and only one bomb around (1, 2).

This strategy is built-in and non-configurable.

### Diff Search

![Diff Search](image/diff.png?raw=true)

According to (1, 1)=1, there is one and only one bomb among "ABC", noted as "ABC"=1.

According to (1, 0)=1, there is one and only one bomb among "AB", noted as "AB"=1. 

In one hand, "ABC"-"AB"="C"; in another hand, "ABC"-"AB"=1-1=0.

Therefore, "C"=0, tile C is safe.

### Reduce Search

![Reduce Search](image/reduce.png?raw=true)

Let's introduce function L(S) to show there's *at least* L(S) bombs in any given tiles set S.

According to "BCD"=2, we can obtain L("BC")=1.

Combined with "ABC"=1, we can obtain "A"=0, tile A is safe.
