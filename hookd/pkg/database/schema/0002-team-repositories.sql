-- Run the entire migration as an atomic operation.
START TRANSACTION ISOLATION LEVEL SERIALIZABLE READ WRITE;

-- Table team_repositories holds information about which repository can deploy to which team's resources.
-- This supports the use of the legacy version in pkg/server/github_handler.go.
CREATE TABLE team_repositories
(
    "team"       varchar not null,
    "repository" varchar not null
);

CREATE INDEX team_repositories_team ON team_repositories (team);
CREATE INDEX team_repositories_repository ON team_repositories (repository);
CREATE UNIQUE INDEX team_repositories_unique ON team_repositories (team, repository);

-- Mark this database migration as completed.
INSERT INTO migrations (version, created)
VALUES (2, now());
COMMIT;
