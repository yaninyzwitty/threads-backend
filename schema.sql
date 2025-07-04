-- cassandra schema

-- schema for users table
CREATE TABLE threads_keyspace.users (
    id bigint PRIMARY KEY,
    username text,
    full_name text,
    email text,
    password text,
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


-- schema for followers

CREATE TABLE IF NOT EXISTS threads_keyspace.followers_by_user  (
    user_id bigint,
    follower_id bigint,
    followed_at timestamp,
    PRIMARY KEY (user_id, follower_id)
);

-- schema for following table

CREATE TABLE IF NOT EXISTS threads_keyspace.following_by_user  (
    user_id bigint,
    following_id bigint,
    following_at timestamp,
    PRIMARY KEY (user_id, following_id)
);

CREATE TABLE IF NOT EXISTS threads_keyspace.follower_counts (
    user_id bigint PRIMARY KEY,
    follower_count counter,
    following_count counter
);




-- create outbox table
CREATE TABLE IF NOT EXISTS threads_keyspace.outbox (
    event_id uuid,
    event_type text,
    payload text, -- for JSONB 
    published boolean,
    PRIMARY KEY (event_id)
    
);

-- create sai on outbox table (published) column
CREATE CUSTOM INDEX ON threads_keyspace.outbox (published)
USING 'StorageAttachedIndex';

