-- Table apikey holds teams' deploy API keys.
CREATE TABLE apikey
(
    "id"       varchar primary key,
    "apikey"   varchar,
    "team"     varchar unique,
    "created"  timestamp,
    "modified" timestamp
);

-- Each row in the deployment table represents a single deployment request.
CREATE TABLE deployment
(
    "id"                varchar primary key,
    "team"              varchar,
    "created"           timestamp,
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
    "created"       timestamp
);
