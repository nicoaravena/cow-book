-- name: GetMember :one
-- GetMember returns the member even if deleted, and pgx.ErrNoRows if the
-- tenant has no such member.
SELECT * FROM members WHERE id = $1;

-- name: LockMember :one
-- LockMember returns the member's deleted_at and keeps a share lock on the row
-- until the transaction ends, so a deletion of the member waits for it. If a
-- deletion holds the row, this waits instead, then reads the row as committed.
-- The foreign key on bookings.member_id takes a weaker lock that a deletion
-- doesn't wait for, so it isn't enough on its own.
SELECT deleted_at FROM members WHERE id = $1 FOR SHARE;

-- name: DeleteMember :execrows
-- DeleteMember marks the member deleted, only if they aren't yet, so of two
-- racing deletions one changes the row and the other 0 rows. The row stays
-- locked until the transaction ends, so a booking insert for the member waits
-- in LockMember.
UPDATE members
SET deleted_at = @deleted_at, updated_at = @updated_at
WHERE id = @id
  AND deleted_at IS NULL;
