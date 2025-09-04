-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS posts (
    title VARCHAR(200) NOT NULL,
    content TEXT,
    user_id INTEGER NOT NULL,
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (FOREIGN) REFERENCES users(id)
);
CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_posts_published ON posts(published);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- REVIEW: Rollback migration - verify data safety

-- No automatic rollback generated
-- TODO: Add manual rollback statements if needed
-- +goose StatementEnd
