-- cassandra schema

-- schema for users table
CREATE TABLE threads_keyspace.users (
    id bigint PRIMARY KEY,
    username text,
    full_name text,
    email text,
    profile_pic_url text,
    is_verified boolean,
    created_at timestamp,
    updated_at timestamp
);

-- create sai on users table (email)
CREATE CUSTOM INDEX ON threads_keyspace.users (email)
USING 'StorageAttachedIndex';




-- schema for user tags

CREATE TABLE threads_keyspace.tag_users (
    tag text,
    user_id bigint,
    PRIMARY KEY (tag, user_id)
);
