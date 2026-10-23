-- Графы, построенные по конспектам: статус обработки, исходный файл, опора на текст.

ALTER TABLE graphs
    ADD COLUMN status      TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('processing', 'ready', 'failed')),
    ADD COLUMN stage       TEXT NOT NULL DEFAULT '',
    ADD COLUMN stage_note  TEXT NOT NULL DEFAULT '',
    ADD COLUMN error       TEXT NOT NULL DEFAULT '',
    ADD COLUMN pages       INT  NOT NULL DEFAULT 0,
    ADD COLUMN source_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN locked_at   TIMESTAMPTZ,
    ADD COLUMN attempts    INT  NOT NULL DEFAULT 0;

CREATE INDEX graphs_processing_idx ON graphs (created_at) WHERE status = 'processing';
CREATE INDEX graphs_source_hash_idx ON graphs (source_hash) WHERE source_hash <> '';

-- Загруженный файл живёт только до конца обработки и удаляется вместе с ней.
CREATE TABLE uploads (
    graph_id   TEXT PRIMARY KEY REFERENCES graphs (id) ON DELETE CASCADE,
    filename   TEXT  NOT NULL,
    mime       TEXT  NOT NULL,
    data       BYTEA NOT NULL
);

-- origin: '' — граф собран вручную, 'explicit' — взято из конспекта, 'inferred' — дополнено моделью.
ALTER TABLE nodes
    ADD COLUMN origin       TEXT NOT NULL DEFAULT '' CHECK (origin IN ('', 'explicit', 'inferred')),
    ADD COLUMN source_quote TEXT NOT NULL DEFAULT '',
    ADD COLUMN source_page  INT  NOT NULL DEFAULT 0,
    ADD COLUMN note         TEXT NOT NULL DEFAULT '',
    ADD COLUMN warning      TEXT NOT NULL DEFAULT '';

ALTER TABLE edges
    ADD COLUMN origin TEXT NOT NULL DEFAULT '' CHECK (origin IN ('', 'explicit', 'inferred'));
