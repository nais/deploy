-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Add cluster field to deployment table.
ALTER TABLE deployment
ADD COLUMN "cluster" VARCHAR NULL;

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (6, now());
COMMIT;
