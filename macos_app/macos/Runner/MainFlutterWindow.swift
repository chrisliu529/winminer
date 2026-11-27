import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  private var windowChannel: FlutterMethodChannel?

  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    self.contentViewController = flutterViewController
    self.title = "Winminer"
    self.setContentSize(NSSize(width: 560, height: 700))
    self.minSize = NSSize(width: 560, height: 520)

    RegisterGeneratedPlugins(registry: flutterViewController)
    let channel = FlutterMethodChannel(
      name: "winminer/window",
      binaryMessenger: flutterViewController.engine.binaryMessenger
    )
    channel.setMethodCallHandler { [weak self] call, result in
      guard call.method == "resizeToFitBoard" else {
        result(FlutterMethodNotImplemented)
        return
      }
      guard
        let arguments = call.arguments as? [String: Any],
        let width = (arguments["width"] as? NSNumber)?.doubleValue,
        let height = (arguments["height"] as? NSNumber)?.doubleValue,
        width.isFinite,
        height.isFinite,
        width > 0,
        height > 0
      else {
        result(
          FlutterError(
            code: "invalid_window_size",
            message: "Window width and height must be positive finite numbers.",
            details: nil
          )
        )
        return
      }
      guard let self else {
        result(
          FlutterError(
            code: "window_unavailable",
            message: "The Winminer window is no longer available.",
            details: nil
          )
        )
        return
      }

      self.resizeWindow(width: CGFloat(width), height: CGFloat(height))
      result(nil)
    }
    windowChannel = channel

    super.awakeFromNib()
  }

  private func resizeWindow(width: CGFloat, height: CGFloat) {
    let minimumSize = NSSize(width: 560, height: 520)
    let visibleFrame = (screen ?? NSScreen.main)?.visibleFrame
    let maximumWidth =
      visibleFrame.map { max(minimumSize.width, $0.width - 32) } ?? width
    let maximumHeight =
      visibleFrame.map { max(minimumSize.height, $0.height - 64) } ?? height
    let targetSize = NSSize(
      width: min(max(width, minimumSize.width), maximumWidth),
      height: min(max(height, minimumSize.height), maximumHeight)
    )
    let oldTop = frame.maxY
    setContentSize(targetSize)

    var updatedFrame = frame
    updatedFrame.origin.y = oldTop - updatedFrame.height
    if let visibleFrame {
      if updatedFrame.width <= visibleFrame.width {
        updatedFrame.origin.x = min(
          max(updatedFrame.origin.x, visibleFrame.minX),
          visibleFrame.maxX - updatedFrame.width
        )
      } else {
        updatedFrame.origin.x = visibleFrame.minX
      }
      if updatedFrame.height <= visibleFrame.height {
        updatedFrame.origin.y = min(
          max(updatedFrame.origin.y, visibleFrame.minY),
          visibleFrame.maxY - updatedFrame.height
        )
      } else {
        updatedFrame.origin.y = visibleFrame.minY
      }
    }
    setFrameOrigin(updatedFrame.origin)
  }
}
