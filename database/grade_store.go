// InfoMark - a platform for managing courses with
//            distributing exercise sheets and testing exercise submissions
// Copyright (C) 2019 ComputerGraphics Tuebingen
//               2020-present InfoMark.org
// Authors: Patrick Wieschollek
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package database

import (
	"database/sql"
	"fmt"

	"github.com/infomark-org/infomark/model"
	"github.com/infomark-org/infomark/symbol"
	"github.com/jmoiron/sqlx"
)

type TeammateGrade struct {
	GradeID              int64  `db:"grade_id"`
	UserID               int64  `db:"user_id"`
	FirstName            string `db:"first_name"`
	LastName             string `db:"last_name"`
	AcquiredPoints       int    `db:"acquired_points"`
	Feedback             string `db:"feedback"`
	IsIndividualOverride bool   `db:"is_individual_override"`
}

type MemberGradeOverride struct {
	UserID         int64
	AcquiredPoints int
	Feedback       string
}

type GradeStore struct {
	db *sqlx.DB
}

func NewGradeStore(db *sqlx.DB) *GradeStore {
	return &GradeStore{
		db: db,
	}
}

// func (s *GradeStore) Get(id int64) (*model.Grade, error) {
//   p := model.Grade{ID: id}
//   err := s.db.Get(&p, "SELECT * FROM grades WHERE id = $1 LIMIT 1;", p.ID)
//   return &p, err
// }

func (s *GradeStore) Get(id int64) (*model.Grade, error) {
	p := model.Grade{ID: id}
	err := s.db.Get(&p, `
SELECT
  g.*,
  s.user_id,
  u.last_name user_last_name,
  u.first_name user_first_name,
  u.email user_email
FROM
  grades g
INNER JOIN submissions s ON g.submission_id = s.id
INNER JOIN users u ON s.user_id = u.id
WHERE
  g.id = $1 LIMIT 1
`, p.ID)
	return &p, err
}

func (s *GradeStore) Create(p *model.Grade) (*model.Grade, error) {
	newID, err := Insert(s.db, "grades", p)
	if err != nil {
		return nil, err
	}
	return s.Get(newID)
}

func (s *GradeStore) UpdatePrivateTestInfo(gradeID int64, log string, status symbol.TestingResult) error {
	_, err := s.db.Exec(`
UPDATE grades
SET
  private_execution_state=$4,
  private_test_log=$2,
  private_test_status=$3
WHERE id IN (
  SELECT g2.id
  FROM grades g1
  INNER JOIN submissions s1 ON g1.submission_id = s1.id
  INNER JOIN submissions s2 ON s1.team_id IS NOT NULL AND s2.team_id = s1.team_id AND s2.task_id = s1.task_id
  INNER JOIN grades g2 ON g2.submission_id = s2.id
  WHERE g1.id = $1
) OR id = $1;
`, gradeID, log, status, symbol.TestingStateFinished)
	return err
}

func (s *GradeStore) UpdatePublicTestInfo(gradeID int64, log string, status symbol.TestingResult) error {
	_, err := s.db.Exec(`
UPDATE grades
SET
  public_execution_state=$4,
  public_test_log=$2,
  public_test_status=$3
WHERE id IN (
  SELECT g2.id
  FROM grades g1
  INNER JOIN submissions s1 ON g1.submission_id = s1.id
  INNER JOIN submissions s2 ON s1.team_id IS NOT NULL AND s2.team_id = s1.team_id AND s2.task_id = s1.task_id
  INNER JOIN grades g2 ON g2.submission_id = s2.id
  WHERE g1.id = $1
) OR id = $1;
`, gradeID, log, status, symbol.TestingStateFinished)
	return err
}

func (s *GradeStore) GetTeammateGrades(gradeID int64) ([]TeammateGrade, error) {
	var results []TeammateGrade
	err := s.db.Select(&results, `
SELECT
  g2.id AS grade_id,
  u.id AS user_id,
  u.first_name,
  u.last_name,
  g2.acquired_points,
  g2.feedback,
  g2.is_individual_override
FROM grades g1
INNER JOIN submissions s1 ON g1.submission_id = s1.id
INNER JOIN submissions s2 ON s1.team_id IS NOT NULL AND s2.team_id = s1.team_id AND s2.task_id = s1.task_id
INNER JOIN grades g2 ON g2.submission_id = s2.id
INNER JOIN users u ON s2.user_id = u.id
WHERE g1.id = $1
ORDER BY u.last_name ASC;`, gradeID)
	return results, err
}

func (s *GradeStore) UpdateWithOverrides(gradeID int64, basePoints int, baseFeedback string, tutorID int64, overrides map[int64]MemberGradeOverride, clearOverrideUserIDs []int64) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	type SubInfo struct {
		SubmissionID int64         `db:"submission_id"`
		TeamID       sql.NullInt64 `db:"team_id"`
		TaskID       int64         `db:"task_id"`
	}
	var sub SubInfo
	err = tx.Get(&sub, `
SELECT s.id AS submission_id, s.team_id, s.task_id
FROM grades g
INNER JOIN submissions s ON g.submission_id = s.id
WHERE g.id = $1 LIMIT 1;`, gradeID)
	if err != nil {
		return err
	}

	if !sub.TeamID.Valid {
		_, err = tx.Exec(`
UPDATE grades
SET acquired_points = $1, feedback = $2, tutor_id = $3, is_individual_override = false, updated_at = NOW()
WHERE id = $4;`, basePoints, baseFeedback, tutorID, gradeID)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	type MemberGradeRow struct {
		GradeID              int64 `db:"grade_id"`
		UserID               int64 `db:"user_id"`
		IsIndividualOverride bool  `db:"is_individual_override"`
	}
	var rows []MemberGradeRow
	err = tx.Select(&rows, `
SELECT g.id AS grade_id, s.user_id, g.is_individual_override
FROM submissions s
INNER JOIN grades g ON g.submission_id = s.id
WHERE s.team_id = $1 AND s.task_id = $2;`, sub.TeamID.Int64, sub.TaskID)
	if err != nil {
		return err
	}

	clearSet := make(map[int64]bool)
	for _, uid := range clearOverrideUserIDs {
		clearSet[uid] = true
	}

	for _, row := range rows {
		if ovr, hasOvr := overrides[row.UserID]; hasOvr {
			_, err = tx.Exec(`
UPDATE grades
SET acquired_points = $1, feedback = $2, tutor_id = $3, is_individual_override = true, updated_at = NOW()
WHERE id = $4;`, ovr.AcquiredPoints, ovr.Feedback, tutorID, row.GradeID)
			if err != nil {
				return err
			}
		} else if clearSet[row.UserID] {
			_, err = tx.Exec(`
UPDATE grades
SET acquired_points = $1, feedback = $2, tutor_id = $3, is_individual_override = false, updated_at = NOW()
WHERE id = $4;`, basePoints, baseFeedback, tutorID, row.GradeID)
			if err != nil {
				return err
			}
		} else if row.IsIndividualOverride {
			continue
		} else {
			_, err = tx.Exec(`
UPDATE grades
SET acquired_points = $1, feedback = $2, tutor_id = $3, is_individual_override = false, updated_at = NOW()
WHERE id = $4;`, basePoints, baseFeedback, tutorID, row.GradeID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *GradeStore) GetForSubmission(id int64) (*model.Grade, error) {
	p := model.Grade{}
	err := s.db.Get(&p, "SELECT * FROM grades WHERE submission_id = $1 LIMIT 1;", id)
	return &p, err
}

func (s *GradeStore) GetOverviewGrades(courseID int64, groupID int64) ([]model.OverviewGrade, error) {
	p := []model.OverviewGrade{}
	err := s.db.Select(&p, `
SELECT
  sum(g.acquired_points) points,
  s.user_id,
  ts.sheet_id,
  sh.name,
  u.first_name user_first_name,
  u.last_name user_last_name,
  u.student_number user_student_number,
  u.email user_email
FROM
  grades g
INNER JOIN submissions s ON g.submission_id = s.id
INNER JOIN tasks t ON s.task_id = t.id
INNER JOIn task_sheet ts ON t.id = ts.task_id
INNER JOIn sheets sh ON ts.sheet_id = sh.id
INNER JOIN sheet_course sc ON ts.sheet_id = sc.sheet_id
INNEr JOIN courses c ON sc.course_id = c.id
INNER JOIN user_course uc ON s.user_id = uc.user_id
INNER JOIN user_group ug ON  s.user_id = ug.user_id
INNER JOIN groups gs ON  ug.group_id = gs.id
INNER JOIN users u ON  s.user_id = u.id
WHERE
  c.ID = $1
AND
  uc.role = 0
AND
  gs.course_id = $1
AND
  uc.course_id = $1
AND
  ($2 = 0 OR gs.id = $2)
GROUP BY
  s.user_id, ts.sheet_id, sh.name, u.first_name, u.last_name, u.student_number, u.email
ORDER BY
  s.user_id
`, courseID, groupID)
	return p, err
}

func (s *GradeStore) GetAllMissingGrades(courseID int64, tutorID int64, groupID int64) ([]model.MissingGrade, error) {
	p := []model.MissingGrade{}

	args := []interface{}{courseID}
	query := `
WITH ranked_teams AS (
  SELECT id, DENSE_RANK() OVER (ORDER BY id ASC) AS team_number
  FROM teams
  WHERE course_id = $1
)
SELECT
  g.*,
  ts.task_id,
  ts.sheet_id,
  sg.course_id,
  s.user_id,
  s.team_id,
  rt.team_number,
  u.last_name user_last_name,
  u.first_name user_first_name,
  u.email user_email
FROM
  grades g
INNER JOIN submissions s ON s.id = g.submission_id
INNER JOIN task_sheet ts ON ts.task_id = s.task_id
INNER JOIN sheet_course sg ON sg.sheet_id = ts.sheet_id
INNER JOIN users u ON s.user_id = u.id
LEFT JOIN ranked_teams rt ON rt.id = s.team_id
LEFT JOIN user_group ug ON ug.user_id = u.id AND ug.group_id IN (SELECT id FROM groups WHERE course_id = $1)
WHERE
  g.feedback like ''
  AND g.acquired_points = 0
  AND sg.course_id = $1
`
	argIdx := 2
	if tutorID > 0 {
		query += fmt.Sprintf(` AND g.tutor_id = $%d`, argIdx)
		args = append(args, tutorID)
		argIdx++
	}
	if groupID > 0 {
		query += fmt.Sprintf(` AND ug.group_id = $%d`, argIdx)
		args = append(args, groupID)
	}
	query += ` ORDER BY g.updated_at DESC;`

	err := s.db.Select(&p, query, args...)
	return p, err
}

func (s *GradeStore) GetAllGradedSubmissions(courseID int64, tutorID int64, groupID int64) ([]model.MissingGrade, error) {
	p := []model.MissingGrade{}

	args := []interface{}{courseID}
	query := `
WITH ranked_teams AS (
  SELECT id, DENSE_RANK() OVER (ORDER BY id ASC) AS team_number
  FROM teams
  WHERE course_id = $1
)
SELECT
  g.*,
  ts.task_id,
  ts.sheet_id,
  sg.course_id,
  s.user_id,
  s.team_id,
  rt.team_number,
  u.last_name user_last_name,
  u.first_name user_first_name,
  u.email user_email
FROM
  grades g
INNER JOIN submissions s ON s.id = g.submission_id
INNER JOIN task_sheet ts ON ts.task_id = s.task_id
INNER JOIN sheet_course sg ON sg.sheet_id = ts.sheet_id
INNER JOIN users u ON s.user_id = u.id
LEFT JOIN ranked_teams rt ON rt.id = s.team_id
LEFT JOIN user_group ug ON ug.user_id = u.id AND ug.group_id IN (SELECT id FROM groups WHERE course_id = $1)
WHERE
  (g.feedback != '' OR g.acquired_points > 0)
  AND sg.course_id = $1
`
	argIdx := 2
	if tutorID > 0 {
		query += fmt.Sprintf(` AND g.tutor_id = $%d`, argIdx)
		args = append(args, tutorID)
		argIdx++
	}
	if groupID > 0 {
		query += fmt.Sprintf(` AND ug.group_id = $%d`, argIdx)
		args = append(args, groupID)
	}
	query += ` ORDER BY g.updated_at DESC;`

	err := s.db.Select(&p, query, args...)
	return p, err
}

func (s *GradeStore) Update(p *model.Grade) error {
	return Update(s.db, "grades", p.ID, p)
}

func (s *GradeStore) GetFiltered(
	courseID int64,
	sheetID int64,
	taskID int64,
	groupID int64,
	userID int64,
	tutorID int64,
	feedback string,
	acquiredPoints int,
	publicTestStatus int,
	privateTestStatus int,
	publicExecutationState int,
	privateExecutationState int,
) ([]model.Grade, error) {

	p := []model.Grade{}
	err := s.db.Select(&p,
		`
SELECT
  g.*, s.user_id,
  u.last_name user_last_name,
  u.first_name user_first_name,
  u.email user_email
FROM
  grades g
INNER JOIN submissions s ON s.id = g.submission_id
INNER JOIN task_sheet ts ON ts.task_id = s.task_id
INNER JOIN sheet_course sc ON sc.sheet_id = ts.sheet_id
INNER JOIN user_group ug ON ug.user_id = s.user_id
INNER JOIN users u ON s.user_id = u.id
WHERE
  course_id = $1
AND
  ug.group_id = $4
AND
  ($2 = 0 OR ts.sheet_id = $2)
AND
  ($3 = 0 OR s.task_id = $3)
AND
  ($5 = 0 OR ug.user_id = $5)
AND
  ($6 = 0 OR tutor_id = $6)
AND
  feedback LIKE $7
AND
  ($8 = -1 OR g.acquired_points = $8)
AND
  ($9 = -1 OR g.public_test_status = $9)
AND
  ($10 = -1 OR g.private_test_status = $10)
AND
  ($11 = -1 OR g.public_execution_state = $11)
AND
  ($12 = -1 OR g.private_execution_state = $12)
  `,
		// AND ($4 = 0 OR ug.group_id = $4)
		courseID,                // $1
		sheetID,                 // $2
		taskID,                  // $3
		groupID,                 // $4
		userID,                  // $5
		tutorID,                 // $6
		feedback,                // $7
		acquiredPoints,          // $8
		publicTestStatus,        // $9
		privateTestStatus,       // $10
		publicExecutationState,  // $11
		privateExecutationState, // $12
	)
	return p, err
}

func (s *GradeStore) IdentifyCourseOfGrade(gradeID int64) (*model.Course, error) {

	course := &model.Course{}
	err := s.db.Get(course,
		`
SELECT
  c.*
FROM
  grades g
INNER JOIN submissions s ON s.id = g.submission_id
INNER JOIN task_sheet ts ON ts.task_id = s.task_id
INNER JOIN sheet_course sc ON sc.sheet_id = ts.sheet_id
INNER JOIN courses c ON sc.course_id = c.id
WHERE
  g.id = $1`,
		gradeID)
	if err != nil {
		return nil, err
	}

	return course, err
}

func (s *GradeStore) IdentifyTaskOfGrade(gradeID int64) (*model.Task, error) {

	task := &model.Task{}
	err := s.db.Get(task,
		`
SELECT
  t.*
FROM
  grades g
INNER JOIN submissions s ON s.id = g.submission_id
INNER JOIN task_sheet ts ON ts.task_id = s.task_id
INNER JOIN tasks t ON ts.task_id = t.id
WHERE
  g.id = $1`,
		gradeID)
	if err != nil {
		return nil, err
	}

	return task, err
}
