-- Platform migration: run once per database, not per tenant.
--
-- platform holds data about operators, never an operator's own data.
-- shared holds extensions every tenant schema uses; each tenant transaction
-- puts it on its search_path after the tenant's schema.

-- +goose Up
CREATE SCHEMA platform;
CREATE SCHEMA shared;

-- btree_gist lets an exclusion constraint compare a plain column (space_id)
-- with = alongside a range with && (2.3).
CREATE EXTENSION btree_gist SCHEMA shared;

CREATE TABLE platform.tenants (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- The slug is the operator's part of every URL. The schema name is built
    -- from id, never from the slug.
    slug       text NOT NULL
               CONSTRAINT tenants_slug_key UNIQUE
               CONSTRAINT tenants_slug_format CHECK (
                   slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
                   AND length(slug) BETWEEN 3 AND 40
               ),
    name       text NOT NULL
               CONSTRAINT tenants_name_not_blank CHECK (btrim(name) <> ''),
    status     text NOT NULL DEFAULT 'provisioning'
               CONSTRAINT tenants_status_valid CHECK (
                   status IN ('provisioning', 'active', 'suspended')
               ),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE platform.tenants;
DROP EXTENSION btree_gist;
DROP SCHEMA shared;
DROP SCHEMA platform;
