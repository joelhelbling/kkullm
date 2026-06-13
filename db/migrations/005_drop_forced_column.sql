-- Transition rules and the force-move feature were removed; no status change is
-- "forced" anymore. Drop the now-unused column from the audit trail.
ALTER TABLE card_events DROP COLUMN forced;
