# Telegram Media Downloader — Quiet library

Approved direction: clean native desktop utility, warm neutral surfaces, forest green actions, content first. Applies to Flutter/macOS, with adaptive Windows/Linux layouts. Source guidance: UI/UX Pro Max minimalism/Swiss style, Flutter semantics/text scaling, UX keyboard focus and loading feedback. The skill's generic marketing-page pattern and blue/amber product palette are not part of this desktop design.

## Tokens

| Role | Light | Dark |
|---|---|---|
| Main surface | #FFFFFF | #1B221E |
| Background | #F6F7F4 | #151B17 |
| Sidebar | #F1F3EF | #171E19 |
| Primary text | #25332E | #E4EBE6 |
| Secondary text | #627067 | #AAB8AE |
| Action | #23664E | #9AD4B3 |
| Text on action | #FFFFFF | #173C2A |
| Selection | #EDF4EF | #283F31 |
| Divider | #E2E7DF | #354239 |
| Control boundary | #7A887E | #83998A |

Semantic tokens are implemented in `lib/ui/quiet_theme.dart`. Use system appearance; no theme picker is required. Dividers are decorative; controls need distinct boundaries/focus rings.

## Typography and geometry

System typography, file names 15px, body 14px, metadata 12px, headings 20/26px. Tabular figures for progress and aligned sizes. Scale with user text settings. Use 4/8px spacing increments, 8px control corners, 14px dialog corners, thin separators, and outline Material icons. Avoid repeating card frames around files. Comfortable file rows expand for long names or larger text.

## Interactions

One primary action: Download N files. Whole-topic/chat downloads are secondary, with scope confirmation. File types live in one persistent multi-select menu. Search clearly applies to loaded results. Keyboard focus remains visible; Command/Ctrl+F focuses file search, Escape dismisses menus. All icon actions need tooltips and semantics. Saved/failed/paused states use text and icons.

The bottom transfer strip shows the active item and honest telemetry. The queue expands inline on sufficiently tall windows; compact windows use a full queue view. Opening/closing the queue preserves browser state and scroll. Keep lists virtualized, loading placeholders stable, cached rows usable, errors actionable and near the affected area. Motion is short, interruptible and respects reduced-motion settings.

## Resizable chat sidebar

The desktop sidebar defaults to 248 logical pixels. Its 8px divider supports dragging between 220 and 420px, with a resize cursor and a subtle forest focus/hover line. Double-click or Enter resets the width. Arrow keys adjust by 16px; Home/End move to the current limits. A secondary-click menu offers narrower, wider and reset controls without dragging. Screen-reader semantics expose the current width and adjustment actions.

Preserve at least 520px for the file pane by temporarily limiting the sidebar on narrower windows. Keep the user's preferred width so widening the window restores it. Compact and large-text layouts retain the drawer. Save only completed user adjustments, independently of window resizing, in a local `ui-settings.json` preference file. Preferences honor `TMM_STATE_DIR`; otherwise they use the platform configuration directory. Invalid/missing values fall back safely, and failed saves leave resizing usable with a brief notice.

## Verification

Check light/dark contrast, file filters, selection, keyboard focus, queue context preservation, long names, large text, 560px compact and normal desktop windows. Render synthetic previews and compile/package macOS. Live Telegram transfers and native VoiceOver/file-picker interaction must be recorded separately from widget/semantics tests.

## Preview and filter extension — 2026-10-27

Keep the Quiet library palette and flat list. Align names after a uniform preview column: 96×54 video images on wide layouts, 64×36 when narrower, centered document glyphs in the same column. Duration is a small contrasting badge; unknown previews keep a play glyph. Do not animate preview placeholders or download full videos to populate them.

Common statuses sit in compact choices on wide screens, becoming one status menu for narrow/larger-text layouts. Advanced filters use a single scrollable dialog with a draft state: Escape/Close cancels and Show files applies. Active restrictions use removable labels that wrap. Sorting and grouping are independent controls. Preserve selection and disclose hidden selected files. Keep library scope in the sidebar; cross-chat/topic filtering belongs in Downloads. The inline download tray now needs at least 1000px pane height and 900px pane width and reserves 400px; smaller windows use the full content pane.
