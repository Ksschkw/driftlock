CREATE TABLE users (id INT, name VARCHAR(100));
CREATE OR REPLACE VIEW active_users AS SELECT 1;
CREATE INDEX idx_users_name ON users (name);
