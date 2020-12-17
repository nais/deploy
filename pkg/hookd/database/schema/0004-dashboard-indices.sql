-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- These indices are neccessary to run the deployment frontend dashboard in a timely manner.
CREATE INDEX deployment_created ON deployment (created);
CREATE INDEX deployment_status_deployment_id ON deployment_status (deployment_id);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (4, now());
COMMIT;
