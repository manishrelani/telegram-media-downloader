import Cocoa
import FlutterMacOS

class MainFlutterWindow: NSWindow {
  override func awakeFromNib() {
    let flutterViewController = FlutterViewController()
    let windowFrame = self.frame
    self.contentViewController = flutterViewController
    self.setFrame(windowFrame, display: true)
    self.title = "Telegram Media Downloader"
    self.minSize = NSSize(width: 760, height: 560)
    if !self.setFrameUsingName("TelegramMediaManagerWindow") {
      let available = NSScreen.main?.visibleFrame.size ?? NSSize(width: 1320, height: 900)
      self.setContentSize(NSSize(width: min(1240, available.width - 40),
                                 height: min(820, available.height - 60)))
      self.center()
    }
    self.setFrameAutosaveName("TelegramMediaManagerWindow")

    RegisterGeneratedPlugins(registry: flutterViewController)

    super.awakeFromNib()
  }
}
