BEGIN;

ALTER TABLE courses
  DROP COLUMN IF EXISTS bonus_mode;

ALTER TABLE exams
  DROP COLUMN IF EXISTS max_points,
  DROP COLUMN IF EXISTS bonus_enabled,
  DROP COLUMN IF EXISTS bonus_percentage;

ALTER TABLE user_exam
  DROP COLUMN IF EXISTS exam_points,
  DROP COLUMN IF EXISTS bonus_points,
  DROP COLUMN IF EXISTS raw_mark;

COMMIT;

