BEGIN;

ALTER TABLE courses
  DROP COLUMN IF EXISTS min_group_preferences,
  DROP COLUMN IF EXISTS group_solver;

COMMIT;

