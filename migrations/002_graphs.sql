-- Несколько независимых графов. Существующие узлы и связи переходят в демо-граф.
CREATE TABLE graphs (
    id         TEXT PRIMARY KEY CHECK (id ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    title      TEXT        NOT NULL,
    section    TEXT        NOT NULL DEFAULT '',
    source     TEXT        NOT NULL DEFAULT '',
    is_demo    BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO graphs (id, title, section, source, is_demo)
SELECT 'demo', 'Электричество', 'Электродинамика', 'Демо-граф', true
WHERE EXISTS (SELECT 1 FROM nodes);

ALTER TABLE nodes ADD COLUMN graph_id TEXT REFERENCES graphs (id) ON DELETE CASCADE;
UPDATE nodes SET graph_id = 'demo';
ALTER TABLE nodes ALTER COLUMN graph_id SET NOT NULL;
ALTER TABLE nodes DROP CONSTRAINT nodes_slug_key;
ALTER TABLE nodes ADD CONSTRAINT nodes_graph_slug_key UNIQUE (graph_id, slug);

ALTER TABLE edges ADD COLUMN graph_id TEXT REFERENCES graphs (id) ON DELETE CASCADE;
UPDATE edges SET graph_id = 'demo';
ALTER TABLE edges ALTER COLUMN graph_id SET NOT NULL;
CREATE INDEX edges_graph_id_idx ON edges (graph_id);
