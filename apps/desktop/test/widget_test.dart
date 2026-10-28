import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/widgets.dart';
import 'package:telegram_media_downloader/main.dart';
import 'support/sidebar_width_store.dart';

void main() {
  testWidgets('main screen displays product title and download workflow', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      TelegramMediaDownloaderApp(
        sidebarWidthStore: FakeSidebarWidthStore(),
        autoConnect: false,
      ),
    );
    await tester.pump();
    expect(find.text('Telegram Media Downloader'), findsOneWidget);
    expect(
      find.image(const AssetImage('assets/branding/logo.png')),
      findsOneWidget,
    );
    expect(find.text('Chats'), findsOneWidget);
    expect(find.text('Download queue'), findsOneWidget);
  });
}
