-- +goose Up

CREATE TABLE tracks (
    -- Track replacement deletes the previous row before inserting its
    -- successor. AUTOINCREMENT prevents SQLite from reusing that deleted
    -- row ID, so a replacement is represented by a distinct Track.
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    hike_id INTEGER NOT NULL
        UNIQUE
        REFERENCES hikes(id)
        ON DELETE CASCADE,

    source_filename TEXT NOT NULL
        DEFAULT '',

    point_count INTEGER NOT NULL
        CHECK(point_count >= 2),

    distance_km REAL NOT NULL
        CHECK(distance_km >= 0),

    elevation_gain_m REAL NOT NULL
        DEFAULT 0
        CHECK(elevation_gain_m >= 0),

    duration_seconds INTEGER
        CHECK(
            duration_seconds IS NULL
            OR duration_seconds >= 0
        ),

    start_lat REAL NOT NULL
        CHECK(start_lat BETWEEN -90 AND 90),

    start_lon REAL NOT NULL
        CHECK(start_lon BETWEEN -180 AND 180),

    end_lat REAL NOT NULL
        CHECK(end_lat BETWEEN -90 AND 90),

    end_lon REAL NOT NULL
        CHECK(end_lon BETWEEN -180 AND 180),

    created_at DATETIME NOT NULL
);


CREATE TABLE track_points (
    track_id INTEGER NOT NULL
        REFERENCES tracks(id)
        ON DELETE CASCADE,

    sequence INTEGER NOT NULL
        CHECK(sequence >= 0),

    segment_index INTEGER NOT NULL
        CHECK(segment_index >= 0),

    latitude REAL NOT NULL
        CHECK(latitude BETWEEN -90 AND 90),

    longitude REAL NOT NULL
        CHECK(longitude BETWEEN -180 AND 180),

    elevation_m REAL,

    recorded_at DATETIME,

    PRIMARY KEY (
        track_id,
        sequence
    )
);


-- +goose Down

DROP TABLE track_points;

DROP TABLE tracks;
