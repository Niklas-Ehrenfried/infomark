BEGIN;

ALTER TABLE courses
  ADD COLUMN group_solver VARCHAR(32) NOT NULL DEFAULT 'optimal',
  ADD COLUMN min_group_preferences INT NOT NULL DEFAULT 3;

COMMIT;

