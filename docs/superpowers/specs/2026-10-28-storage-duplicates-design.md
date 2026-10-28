# Storage failures and stable duplicate detection

The user approved fixing the two final-review issues, with TDD, edge coverage and a new macOS app. Execute this approved scope inline; no new UI or security/deployment work.

Filesystem failures during finalization must preserve their cause and stop later work. The organizer will return both its recorded outcome and a typed storage error immediately for destination inspection, journaling or move failures. Missing/ambiguous upstream output remains a distinct per-file error. Successfully moved files and recoverable staging remain intact; retry reconciles the existing manifest.

Chat folders retain their friendly current format. Resolve an existing folder by stable chat ID, using recorded jobs and direct-child manifests in the chosen save root, before creating a new name. Reuse, never rename or migrate, an existing folder. Reject conflicting multiple roots rather than guessing/merging. Ignore other chat IDs and other save roots. Retain original paths on resume. Serialize new folder choice within a service so simultaneously queued renamed-chat jobs do not split roots.

Captions do not identify downloaded media. Match the existing attachment key, saved path and size; persist an additive exact content ID from raw Telegram document/photo IDs to detect known replacements even when the filename/size matches. Old manifests without content IDs continue to work; their original media version cannot be retroactively proved. Never infer missing files are complete or overwrite old files on replacement.

Compare minimal fixes above with automatic folder migration or caption-driven renaming: those alternatives create unnecessary filesystem changes and recovery risk. Preserve current files and metadata compatibility instead.

Acceptance covers journal/move/inspection failures, partial success and retry, missing/ambiguous input, title/caption edits, fresh-service/history-independent folder discovery, known content changes, missing/wrong-size files, identity isolation, conflicting roots, legacy manifests and original-destination resume.
