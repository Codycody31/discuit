-- Per community automod configuration
create table if not exists automod_config(
    id int auto_increment not null,
    community_id binary (12) null,
    active boolean not null,
    config_json json,
    
    primary key (id),
    unique (community_id),
    foreign key (community_id) references communities (id)
);

-- Used to store flags on content such as posts, comments, etc.
create table if not exists automod_flag(
    id int auto_increment not null,

    -- How we know what it was
    content_table varchar(255) not null,
    content_id binary (12) not null,

    -- Automod options
    reason int not null,
    action_taken int not null default 0,

    -- Mod/Admin options
    cleared boolean not null default false,
    notes text,
    
    primary key (id)
);

-- Add columns to existing tables to support automod
alter table comments add column automod_flag_id int null;
alter table communities add column automod_config_id int null;
alter table posts add column automod_flag_id int null;
alter table users add column automod_flag_id int null;