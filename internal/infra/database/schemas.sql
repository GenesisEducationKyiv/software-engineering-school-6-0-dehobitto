CREATE TABLE IF NOT EXISTS subscriptions (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    confirmed BOOLEAN DEFAULT false,
    token VARCHAR(255),
    last_seen_tag VARCHAR(100),
    UNIQUE (email, repo)
);