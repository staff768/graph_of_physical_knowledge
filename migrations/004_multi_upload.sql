-- Один граф может строиться из нескольких файлов: страницы рукописного конспекта фотографиями.
ALTER TABLE uploads ADD COLUMN position INT NOT NULL DEFAULT 0;
ALTER TABLE uploads DROP CONSTRAINT uploads_pkey;
ALTER TABLE uploads ADD PRIMARY KEY (graph_id, position);
