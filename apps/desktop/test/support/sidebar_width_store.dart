import 'package:telegram_media_downloader/services/sidebar_width_store.dart';

class FakeSidebarWidthStore implements SidebarWidthStore {
  FakeSidebarWidthStore([this.width]);
  double? width;
  int writes = 0;
  @override
  Future<double?> loadWidth() async => width;
  @override
  Future<void> saveWidth(double value) async {
    width = value;
    writes++;
  }
}
