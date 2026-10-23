-- +goose Up

CREATE TABLE tags (
    id INTEGER PRIMARY KEY,

    name TEXT NOT NULL
        UNIQUE
        CHECK(length(trim(name)) > 0),

    created_at DATETIME NOT NULL
);


CREATE TABLE hike_tags (
    hike_id INTEGER NOT NULL
        REFERENCES hikes(id)
        ON DELETE CASCADE,

    tag_id INTEGER NOT NULL
        REFERENCES tags(id)
        ON DELETE CASCADE,

    PRIMARY KEY (
        hike_id,
        tag_id
    )
);


CREATE INDEX idx_hike_tags_tag_id_hike_id
    ON hike_tags (
        tag_id,
        hike_id
    );


-- +goose Down

DROP INDEX idx_hike_tags_tag_id_hike_id;

DROP TABLE hike_tags;

DROP TABLE tags;
