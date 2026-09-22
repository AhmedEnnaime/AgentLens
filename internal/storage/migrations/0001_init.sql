CREATE TABLE traces (
	id                TEXT PRIMARY KEY,
	agent             TEXT NOT NULL,
	root_session_id   TEXT NOT NULL,
	project_directory TEXT NOT NULL,
	start_time        INTEGER NOT NULL,
	end_time          INTEGER,
	content_sha256    TEXT NOT NULL,
	header            TEXT NOT NULL
);
CREATE INDEX idx_traces_project_start ON traces (project_directory, start_time DESC, id);

CREATE TABLE spans (
	trace_id TEXT NOT NULL REFERENCES traces (id),
	id       TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	blob     TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq)
);

CREATE TABLE events (
	trace_id TEXT NOT NULL REFERENCES traces (id),
	id       TEXT NOT NULL,
	span_id  TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	blob     TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq),
	FOREIGN KEY (trace_id, span_id) REFERENCES spans (trace_id, id)
);

CREATE TABLE raw_events (
	trace_id       TEXT NOT NULL,
	id             TEXT NOT NULL,
	seq            INTEGER NOT NULL,
	payload        BLOB NOT NULL,
	payload_sha256 TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq)
);

CREATE TABLE labels (
	trace_id   TEXT NOT NULL REFERENCES traces (id),
	key        TEXT NOT NULL,
	value      TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (trace_id, key)
);

CREATE TRIGGER raw_events_immutable_update BEFORE UPDATE ON raw_events
BEGIN SELECT RAISE(ABORT, 'raw_events is immutable'); END;
CREATE TRIGGER raw_events_immutable_delete BEFORE DELETE ON raw_events
BEGIN SELECT RAISE(ABORT, 'raw_events is immutable'); END;
