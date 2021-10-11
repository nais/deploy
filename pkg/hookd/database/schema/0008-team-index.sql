-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Enable fast lookups on team
CREATE INDEX deployment_team ON deployment (team);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (8, now());
COMMIT;
