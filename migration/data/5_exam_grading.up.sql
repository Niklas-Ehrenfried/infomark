BEGIN;

-- Add global bonus_mode to courses:
-- 0: Disabled, 1: Auto from Worksheets, 2: Manual
ALTER TABLE courses
  ADD COLUMN bonus_mode INT NOT NULL DEFAULT 1;

-- Add max_points, bonus_enabled, bonus_percentage to exams:
ALTER TABLE exams
  ADD COLUMN max_points INT NOT NULL DEFAULT 100,
  ADD COLUMN bonus_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN bonus_percentage INT NOT NULL DEFAULT 10;

-- Add exam_points, bonus_points, raw_mark to user_exam:
ALTER TABLE user_exam
  ADD COLUMN exam_points INT NOT NULL DEFAULT 0,
  ADD COLUMN bonus_points INT NOT NULL DEFAULT 0,
  ADD COLUMN raw_mark TEXT NOT NULL DEFAULT '';

COMMIT;

