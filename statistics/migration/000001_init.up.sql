CREATE TABLE events (
                        id SERIAL PRIMARY KEY,
                        event_type VARCHAR(10) NOT NULL CHECK (event_type IN ('like', 'view', 'comment')),
                        user_id INTEGER NOT NULL,
                        post_id INTEGER NOT NULL,
                        comment_text TEXT,
                        created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_events_post_id ON events(post_id);
CREATE INDEX idx_events_user_id ON events(user_id);
CREATE INDEX idx_events_created_at ON events(created_at);