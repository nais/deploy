-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Table apikey holds teams' deploy API keys.
-- A team can have many API keys, with each key having its own expiry time.
CREATE TABLE apikey
(
    "key"           varchar primary key      not null,
    "team"          varchar                  not null,
    "team_azure_id" varchar                  not null,
    "created"       timestamp with time zone not null,
    "expires"       timestamp with time zone null
);

CREATE INDEX apikey_team_index ON apikey (team);
CREATE INDEX apikey_team_azure_id_index ON apikey (team_azure_id);

-- Each row in the deployment table represents a single deployment request.
CREATE TABLE deployment
(
    "id"                varchar primary key      not null,
    "team"              varchar                  not null,
    "created"           timestamp with time zone not null,
    "github_id"         int unique               null,
    "github_repository" varchar                  null
);

-- A row is recorded in deployment_status for each state change in a deployment.
CREATE TABLE deployment_status
(
    "id"            varchar primary key                not null,
    "deployment_id" varchar references deployment (id) not null,
    "status"        varchar                            not null,
    "message"       varchar                            not null,
    "github_id"     int                                null,
    "created"       timestamp with time zone           not null
);

-- Database migration
CREATE TABLE migrations
(
    "version" int primary key          not null,
    "created" timestamp with time zone not null
);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (1, now());
COMMIT;
