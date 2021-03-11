-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Add cluster field to deployment table.
ALTER TABLE deployment
ADD COLUMN "state" VARCHAR NULL;

-- Enable fast lookups on cluster and state
CREATE INDEX deployment_state ON deployment (state);
CREATE INDEX deployment_cluster ON deployment (cluster);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (7, now());
COMMIT;
