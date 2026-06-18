PRAGMA secure_delete = ON;

CREATE TABLE IF NOT EXISTS pi_meta (
    key TEXT PRIMARY KEY,
    value TEXT
);

INSERT OR IGNORE INTO pi_meta(key, value) VALUES ('schema_version', '3.4');

CREATE TABLE IF NOT EXISTS pi_chats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL,
    version INTEGER,
    cwd TEXT,
    name TEXT,
    session_file TEXT NOT NULL,
    parent_session_file TEXT,
    parent_chat_id INTEGER,
    current_leaf_id TEXT,
    repo_root TEXT,
    file_size INTEGER,
    file_mtime_ms INTEGER,
    header_hash TEXT,
    file_dev INTEGER,
    file_ino INTEGER,
    content_hash TEXT,
    last_full_verify_at DATETIME,
    file_deleted_at DATETIME,
    synced_seq INTEGER NOT NULL DEFAULT -1,
    synced_byte_offset INTEGER NOT NULL DEFAULT 0,
    last_synced_at DATETIME,
    sync_status TEXT NOT NULL DEFAULT 'idle',
    sync_error TEXT,
    last_ingest_started_at DATETIME,
    last_ingest_completed_at DATETIME,
    provider TEXT,
    model TEXT,
    first_user_text TEXT,
    last_user_text TEXT,
    message_count INTEGER NOT NULL DEFAULT 0,
    tool_call_count INTEGER NOT NULL DEFAULT 0,
    file_ref_count INTEGER NOT NULL DEFAULT 0,
    last_message_at TEXT,
    raw_header TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pi_chats_uuid ON pi_chats(uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pi_chats_file ON pi_chats(session_file);
CREATE INDEX IF NOT EXISTS idx_pi_chats_parent ON pi_chats(parent_chat_id);
CREATE INDEX IF NOT EXISTS idx_pi_chats_parent_file ON pi_chats(parent_session_file);
CREATE INDEX IF NOT EXISTS idx_pi_chats_repo ON pi_chats(repo_root);
CREATE INDEX IF NOT EXISTS idx_pi_chats_status ON pi_chats(sync_status);
CREATE INDEX IF NOT EXISTS idx_pi_chats_updated ON pi_chats(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_pi_chats_cwd_updated ON pi_chats(cwd, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_pi_chats_last_msg ON pi_chats(last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_pi_chats_live ON pi_chats(updated_at DESC) WHERE file_deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pi_chats_name ON pi_chats(name) WHERE name IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pi_chats_model ON pi_chats(model) WHERE model IS NOT NULL;

CREATE TABLE IF NOT EXISTS pi_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    entry_id TEXT NOT NULL,
    parent_entry_id TEXT,
    seq INTEGER NOT NULL,
    type TEXT NOT NULL,
    role TEXT,
    model TEXT,
    provider TEXT,
    text TEXT,
    raw_line TEXT NOT NULL,
    raw_hash TEXT NOT NULL,
    timestamp TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pi_messages_entry ON pi_messages(chat_id, entry_id);
CREATE INDEX IF NOT EXISTS idx_pi_messages_parent ON pi_messages(chat_id, parent_entry_id);
CREATE INDEX IF NOT EXISTS idx_pi_messages_seq ON pi_messages(chat_id, seq);
CREATE INDEX IF NOT EXISTS idx_pi_messages_type ON pi_messages(chat_id, type);

CREATE TABLE IF NOT EXISTS pi_tool_calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    entry_id TEXT NOT NULL,
    block_index INTEGER NOT NULL,
    tool_call_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    arguments_json TEXT NOT NULL,
    result_message_id INTEGER,
    result_entry_id TEXT,
    is_error INTEGER,
    result_text TEXT,
    seq INTEGER NOT NULL,
    timestamp TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pi_tool_calls_id ON pi_tool_calls(chat_id, tool_call_id);
CREATE INDEX IF NOT EXISTS idx_pi_tool_calls_name ON pi_tool_calls(chat_id, tool_name);
CREATE INDEX IF NOT EXISTS idx_pi_tool_calls_entry ON pi_tool_calls(chat_id, entry_id);
CREATE INDEX IF NOT EXISTS idx_pi_tool_calls_msg ON pi_tool_calls(message_id);

CREATE TABLE IF NOT EXISTS pi_file_refs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER NOT NULL,
    message_id INTEGER NOT NULL,
    entry_id TEXT NOT NULL,
    tool_call_id TEXT,
    source TEXT NOT NULL,
    op TEXT NOT NULL,
    tool_name TEXT,
    raw_path TEXT NOT NULL,
    abs_path TEXT,
    repo_root TEXT,
    file_path_rel TEXT,
    cwd_rel_path TEXT,
    confidence TEXT NOT NULL DEFAULT 'high',
    timestamp TEXT NOT NULL,
    CHECK (source IN ('tool_call','compaction','branch_summary')),
    CHECK (op IN ('read','edit','write','compaction_read','compaction_modified','branch_summary_read','branch_summary_modified')),
    CHECK (confidence IN ('high','heuristic'))
);

CREATE INDEX IF NOT EXISTS idx_pi_file_refs_rel ON pi_file_refs(repo_root, file_path_rel, op, chat_id);
CREATE INDEX IF NOT EXISTS idx_pi_file_refs_time ON pi_file_refs(repo_root, file_path_rel, op, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_pi_file_refs_abs_time ON pi_file_refs(abs_path, op, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_pi_file_refs_relpath ON pi_file_refs(file_path_rel);
CREATE INDEX IF NOT EXISTS idx_pi_file_refs_chat ON pi_file_refs(chat_id);
CREATE INDEX IF NOT EXISTS idx_pi_file_refs_tc ON pi_file_refs(chat_id, tool_call_id);

CREATE TABLE IF NOT EXISTS pi_sync_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id INTEGER,
    session_file TEXT NOT NULL,
    event_type TEXT NOT NULL,
    detected_at DATETIME NOT NULL,
    old_size INTEGER,
    new_size INTEGER,
    old_mtime_ms INTEGER,
    new_mtime_ms INTEGER,
    old_hash TEXT,
    new_hash TEXT,
    old_seq INTEGER,
    new_seq INTEGER,
    note TEXT
);

CREATE INDEX IF NOT EXISTS idx_pi_sync_events_chat ON pi_sync_events(chat_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_pi_sync_events_file ON pi_sync_events(session_file, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_pi_sync_events_type ON pi_sync_events(event_type, detected_at DESC);

CREATE VIRTUAL TABLE IF NOT EXISTS fts_pi_messages USING fts5(
    text,
    chat_id UNINDEXED,
    content='pi_messages'
);

CREATE TRIGGER IF NOT EXISTS pi_messages_ai AFTER INSERT ON pi_messages BEGIN
    INSERT INTO fts_pi_messages(rowid, text, chat_id) VALUES (new.id, new.text, new.chat_id);
END;

CREATE TRIGGER IF NOT EXISTS pi_messages_ad AFTER DELETE ON pi_messages BEGIN
    INSERT INTO fts_pi_messages(fts_pi_messages, rowid, text, chat_id)
        VALUES ('delete', old.id, old.text, old.chat_id);
END;

CREATE TRIGGER IF NOT EXISTS pi_messages_au AFTER UPDATE ON pi_messages BEGIN
    INSERT INTO fts_pi_messages(fts_pi_messages, rowid, text, chat_id)
        VALUES ('delete', old.id, old.text, old.chat_id);
    INSERT INTO fts_pi_messages(rowid, text, chat_id) VALUES (new.id, new.text, new.chat_id);
END;
