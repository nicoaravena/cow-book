-- Step 2.1: the tenants table accepts a good row and refuses bad ones.
-- Run against the compose database after `just migrate-platform`:
--
--   docker compose exec -T db psql -U cowbook -d cowbook < examples/2.1-tenants.sql
--
-- Everything runs in one transaction that is rolled back, so nothing is left
-- behind. ON_ERROR_ROLLBACK makes psql wrap each statement in a savepoint, so
-- a refused statement doesn't abort the ones after it.

\set ON_ERROR_ROLLBACK on
\set ECHO queries
BEGIN;

-- Accepted: id, status and created_at come from their defaults.
INSERT INTO platform.tenants (slug, name) VALUES ('north-hub', 'North Hub')
RETURNING id, slug, name, status;

-- Refused: the slug is taken (tenants_slug_key).
INSERT INTO platform.tenants (slug, name) VALUES ('north-hub', 'Another North Hub');

-- Refused: capitals and a space (tenants_slug_format).
INSERT INTO platform.tenants (slug, name) VALUES ('Bad Slug', 'Bad Slug');

-- Refused: too short (tenants_slug_format).
INSERT INTO platform.tenants (slug, name) VALUES ('ab', 'Ab');

-- Refused: not a known status (tenants_status_valid).
INSERT INTO platform.tenants (slug, name, status) VALUES ('south-hub', 'South Hub', 'deleted');

-- Refused: blank name (tenants_name_not_blank).
INSERT INTO platform.tenants (slug, name) VALUES ('east-hub', '   ');

-- Refused: ids come from the identity, never from the caller.
INSERT INTO platform.tenants (id, slug, name) VALUES (99, 'west-hub', 'West Hub');

-- Only the first row made it.
SELECT id, slug, status FROM platform.tenants ORDER BY id;

ROLLBACK;

-- The schemas and the extension the migration created.
\dn
\dx btree_gist
