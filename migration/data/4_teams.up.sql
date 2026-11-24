BEGIN;

ALTER TABLE courses
  ADD COLUMN teams_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN team_formation_open BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN max_team_size INT NOT NULL DEFAULT 2,
  ADD COLUMN enforce_same_exercise_group BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE sheets
  ADD COLUMN allow_teams BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS teams (
  id SERIAL NOT NULL PRIMARY KEY,
  created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
  updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
  course_id INT NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
  invite_token VARCHAR(32) UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_teams_course ON teams (course_id);

CREATE TABLE IF NOT EXISTS team_members (
  id SERIAL NOT NULL PRIMARY KEY,
  team_id INT NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
  user_id INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  course_id INT NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
  joined_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
  consented_at TIMESTAMP NULL,
  UNIQUE (user_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members (team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members (user_id);

ALTER TABLE submissions
  ADD COLUMN team_id INT NULL REFERENCES teams (id) ON DELETE SET NULL,
  ADD COLUMN submitted_by_user_id INT NULL REFERENCES users (id) ON DELETE SET NULL,
  ADD COLUMN is_final BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_submissions_team ON submissions (team_id);

ALTER TABLE grades
  ADD COLUMN is_individual_override BOOLEAN NOT NULL DEFAULT FALSE;

COMMIT;
