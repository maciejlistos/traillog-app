-- +goose Up

CREATE TABLE hikes (
    id INTEGER PRIMARY KEY,

    name TEXT NOT NULL
        CHECK(length(trim(name)) > 0),

    location TEXT NOT NULL
        CHECK(length(trim(location)) > 0),

    description TEXT NOT NULL
        DEFAULT '',

    distance_km REAL
        CHECK(
            distance_km IS NULL
            OR distance_km >= 0
        ),

    status TEXT NOT NULL
        DEFAULT 'planned'
        CHECK(
            status IN (
                'planned',
                'visited'
            )
        ),

    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- +goose Down

DROP TABLE hikes;
