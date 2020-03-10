-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Table apikey holds teams' deploy API keys.
-- A team can have many API keys, with each key having its own expiry time.
CREATE TABLE apikey
(
    "id"      varchar primary key,
    "apikey"  varchar,
    "team"    varchar,
    "created" timestamp with time zone,
    "expires" timestamp with time zone
);

-- Index teams for quick lookups.
CREATE INDEX apikey_team_index ON apikey (team);

-- Each row in the deployment table represents a single deployment request.
CREATE TABLE deployment
(
    "id"                varchar primary key,
    "team"              varchar,
    "created"           timestamp with time zone,
    "github_id"         int unique,
    "github_repository" varchar
);

-- A row is recorded in deployment_status for each state change in a deployment.
CREATE TABLE deployment_status
(
    "id"            varchar primary key,
    "deployment_id" varchar references deployment (id),
    "status"        varchar,
    "message"       varchar,
    "github_id"     int,
    "created"       timestamp with time zone
);

-- Database migration
CREATE TABLE migrations
(
    "version" int primary key,
    "created" timestamp with time zone
);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (1, now());
COMMIT;