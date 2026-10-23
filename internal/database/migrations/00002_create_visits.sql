-- +goose Up

CREATE TABLE visits (
    id INTEGER PRIMARY KEY,

    hike_id INTEGER NOT NULL
        REFERENCES hikes(id)
        ON DELETE CASCADE,

    visited_at DATETIME NOT NULL,

    weather TEXT NOT NULL
        DEFAULT '',

    notes TEXT NOT NULL
        DEFAULT '',

    rating INTEGER
        CHECK (
            rating IS NULL
            OR rating BETWEEN 1 AND 5
        ),

    created_at DATETIME NOT NULL
);

CREATE INDEX idx_visits_hike_id_visited_at
    ON visits (
        hike_id,
        visited_at DESC,
        id DESC
    );


-- +goose Down

DROP INDEX idx_visits_hike_id_visited_at;

DROP TABLE visits;
