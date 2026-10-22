CREATE TABLE IF NOT EXISTS nodes (
    id      BIGSERIAL PRIMARY KEY,
    slug    TEXT NOT NULL UNIQUE,
    type    TEXT NOT NULL CHECK (type IN ('law', 'quantity', 'formula', 'phenomenon')),
    title   TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    formula TEXT NOT NULL DEFAULT '',
    unit    TEXT NOT NULL DEFAULT '',
    body    TEXT NOT NULL DEFAULT '',
    example TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS edges (
    id          BIGSERIAL PRIMARY KEY,
    from_id     BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    to_id       BIGINT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    type        TEXT   NOT NULL CHECK (type IN ('formulates', 'used_in', 'derives', 'explains', 'causes', 'defines')),
    description TEXT   NOT NULL DEFAULT '',
    UNIQUE (from_id, to_id, type),
    CHECK (from_id <> to_id)
);

CREATE INDEX IF NOT EXISTS edges_to_id_idx ON edges (to_id);
CREATE INDEX IF NOT EXISTS edges_from_id_idx ON edges (from_id);
CREATE INDEX IF NOT EXISTS nodes_title_lower_idx ON nodes (lower(title));
