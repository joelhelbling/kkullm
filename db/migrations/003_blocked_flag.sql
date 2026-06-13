-- blocked becomes an orthogonal flag a card carries while sitting in its real
-- status column, rather than a status of its own. Comments gain a "kind" so
-- block/unblock actions can leave a tagged note in the card timeline.
ALTER TABLE cards ADD COLUMN blocked INTEGER NOT NULL DEFAULT 0;
ALTER TABLE comments ADD COLUMN kind TEXT NOT NULL DEFAULT '';

-- Data migration (lossy, deliberate): existing rows whose status is 'blocked'
-- lose their prior lifecycle position (there is no audit trail yet), so they
-- land in 'todo' with the flag set. 'todo' is the intentional default.
UPDATE cards SET blocked = 1, status = 'todo' WHERE status = 'blocked';
