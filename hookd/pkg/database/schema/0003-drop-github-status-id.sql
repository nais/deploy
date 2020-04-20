-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- This field has never been used and we don't intend to use it anyway.
ALTER TABLE deployment_status
    DROP github_id;

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (3, now());
COMMIT;
