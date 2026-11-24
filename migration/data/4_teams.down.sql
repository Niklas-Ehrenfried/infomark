BEGIN;

ALTER TABLE grades
  DROP COLUMN IF EXISTS is_individual_override;

DROP INDEX IF EXISTS idx_submissions_team;

ALTER TABLE submissions
  DROP COLUMN IF EXISTS team_id,
  DROP COLUMN IF EXISTS submitted_by_user_id,
  DROP COLUMN IF EXISTS is_final;

DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;

ALTER TABLE sheets
  DROP COLUMN IF EXISTS allow_teams;

ALTER TABLE courses
  DROP COLUMN IF EXISTS teams_enabled,
  DROP COLUMN IF EXISTS team_formation_open,
  DROP COLUMN IF EXISTS max_team_size,
  DROP COLUMN IF EXISTS enforce_same_exercise_group;

COMMIT;
