-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Table deployment_resource holds the identifiers of resources touched by this deployment.
CREATE TABLE deployment_resource
(
    "id"            varchar primary key                not null,
    "deployment_id" varchar references deployment (id) not null,
    "index"         int                                not null,
    "group"         varchar                            not null,
    "version"       varchar                            not null,
    "kind"          varchar                            not null,
    "name"          varchar                            not null,
    "namespace"     varchar                            not null
);

CREATE INDEX deployment_resource_deployment_id ON deployment_resource (deployment_id);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (5, now());
COMMIT;
