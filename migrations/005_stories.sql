-- Связные рассказы по цепочкам, написанные моделью. Кэш: при перестройке графа сбрасывается.
CREATE TABLE stories (
    graph_id   TEXT NOT NULL REFERENCES graphs (id) ON DELETE CASCADE,
    slug       TEXT NOT NULL,
    text       TEXT NOT NULL,
    model      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (graph_id, slug)
);
