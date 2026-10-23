-- Владелец графа — хэш личного ключа из cookie браузера (личное пространство без регистрации).
-- У демо-графа и графов, построенных до этой миграции, владельца нет: их видят все.
ALTER TABLE graphs ADD COLUMN owner TEXT;
CREATE INDEX graphs_owner_idx ON graphs (owner);
