-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Remove no longer used Azure column / index
DROP INDEX apikey_team_azure_id_index;
ALTER TABLE apikey DROP COLUMN "team_azure_id";

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (9, now());
COMMIT;
