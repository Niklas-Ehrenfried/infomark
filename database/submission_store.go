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
	"errors"

	null "github.com/guregu/null/v6"
	"github.com/infomark-org/infomark/model"
	"github.com/jmoiron/sqlx"
)

type SubmissionStore struct {
	db *sqlx.DB
}

func NewSubmissionStore(db *sqlx.DB) *SubmissionStore {
	return &SubmissionStore{
		db: db,
	}
}

func (s *SubmissionStore) Get(submissionID int64) (*model.Submission, error) {
	p := model.Submission{ID: submissionID}
	err := s.db.Get(&p, `SELECT * FROM submissions WHERE id = $1 LIMIT 1;`, p.ID)
	return &p, err
}

func (s *SubmissionStore) GetByUserAndTask(userID int64, taskID int64) (*model.Submission, error) {
	p := model.Submission{}
	err := s.db.Get(&p, `
SELECT
  *
FROM
  submissions
WHERE
  user_id = $1
AND
  task_id = $2
LIMIT 1;`,
		userID, taskID)
	return &p, err
}

func (s *SubmissionStore) Create(p *model.Submission) (*model.Submission, error) {
	newID, err := Insert(s.db, "submissions", p)
	if err != nil {
		return nil, err
	}
	return s.Get(newID)
}

func (s *SubmissionStore) Update(p *model.Submission) error {
	return Update(s.db, "submissions", p.ID, p)
}

func (s *SubmissionStore) GetForTeamAndTask(teamID, taskID int64) (*model.Submission, error) {
	p := model.Submission{}
	err := s.db.Get(&p, `
SELECT * FROM submissions
WHERE team_id = $1 AND task_id = $2
ORDER BY id DESC LIMIT 1;`, teamID, taskID)
	return &p, err
}

func (s *SubmissionStore) GetSubmissionsForTeamAndTask(teamID, taskID int64) ([]model.Submission, error) {
	p := []model.Submission{}
	err := s.db.Select(&p, `
SELECT * FROM submissions
WHERE team_id = $1 AND task_id = $2
ORDER BY user_id ASC;`, teamID, taskID)
	return p, err
}

// PrepareUploadSubmissions atomically creates or updates submission and grade rows for all target users in a single transaction.
func (s *SubmissionStore) PrepareUploadSubmissions(
	courseID, taskID, usedUserID int64,
	targetUserIDs []int64,
	teamID null.Int,
	isFinal bool,
	defaultPublicTestLog, defaultPrivateTestLog string,
) (*model.Submission, *model.Grade, []int64, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var uploaderSubmission *model.Submission
	var uploaderGrade *model.Grade
	var otherSubmissionIDs []int64

	for _, targetUID := range targetUserIDs {
		sub := &model.Submission{}
		getErr := tx.Get(sub, `SELECT * FROM submissions WHERE user_id = $1 AND task_id = $2 LIMIT 1 FOR UPDATE;`, targetUID, taskID)
		if getErr != nil {
			if !errors.Is(getErr, sql.ErrNoRows) {
				return nil, nil, nil, getErr
			}

			sub = &model.Submission{
				UserID:            targetUID,
				TaskID:            taskID,
				TeamID:            teamID,
				SubmittedByUserID: null.IntFrom(usedUserID),
				IsFinal:           isFinal,
			}
			newSubID, err := Insert(tx, "submissions", sub)
			if err != nil {
				return nil, nil, nil, err
			}
			if err := tx.Get(sub, `SELECT * FROM submissions WHERE id = $1 LIMIT 1;`, newSubID); err != nil {
				return nil, nil, nil, err
			}

			assignedTutorID := int64(1)
			var groupTutorID int64
			if err := tx.Get(&groupTutorID, `
SELECT g.tutor_id
FROM user_group ug
INNER JOIN groups g ON ug.group_id = g.id
WHERE ug.user_id = $1 AND g.course_id = $2
LIMIT 1;`, targetUID, courseID); err == nil && groupTutorID > 0 {
				assignedTutorID = groupTutorID
			}

			grade := &model.Grade{
				PublicExecutionState:  0,
				PrivateExecutionState: 0,
				PublicTestLog:         defaultPublicTestLog,
				PrivateTestLog:        defaultPrivateTestLog,
				PublicTestStatus:      0,
				PrivateTestStatus:     0,
				AcquiredPoints:        0,
				Feedback:              "",
				TutorID:               assignedTutorID,
				SubmissionID:          sub.ID,
				IsIndividualOverride:  false,
			}
			newGradeID, err := Insert(tx, "grades", grade)
			if err != nil {
				return nil, nil, nil, err
			}
			if err := tx.Get(grade, `SELECT * FROM grades WHERE id = $1 LIMIT 1;`, newGradeID); err != nil {
				return nil, nil, nil, err
			}

			if targetUID == usedUserID {
				uploaderSubmission = sub
				uploaderGrade = grade
			} else {
				otherSubmissionIDs = append(otherSubmissionIDs, sub.ID)
			}
		} else {
			sub.TeamID = teamID
			sub.SubmittedByUserID = null.IntFrom(usedUserID)
			sub.IsFinal = isFinal
			if err := Update(tx, "submissions", sub.ID, sub); err != nil {
				return nil, nil, nil, err
			}

			grade := &model.Grade{}
			if err := tx.Get(grade, `SELECT * FROM grades WHERE submission_id = $1 LIMIT 1 FOR UPDATE;`, sub.ID); err != nil {
				return nil, nil, nil, err
			}

			grade.PublicExecutionState = 0
			grade.PrivateExecutionState = 0
			grade.PublicTestLog = defaultPublicTestLog
			grade.PrivateTestLog = defaultPrivateTestLog
			grade.AcquiredPoints = 0
			grade.Feedback = ""
			grade.IsIndividualOverride = false
			if err := Update(tx, "grades", grade.ID, grade); err != nil {
				return nil, nil, nil, err
			}

			if targetUID == usedUserID {
				uploaderSubmission = sub
				uploaderGrade = grade
			} else {
				otherSubmissionIDs = append(otherSubmissionIDs, sub.ID)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, nil, err
	}

	return uploaderSubmission, uploaderGrade, otherSubmissionIDs, nil
}

func (s *SubmissionStore) GetFiltered(filterCourseID, filterGroupID, filterUserID, filterSheetID, filterTaskID int64) ([]model.Submission, error) {

	p := []model.Submission{}
	err := s.db.Select(&p,
		`
SELECT
  s.*
FROM
  submissions s
INNER JOIN user_group ug ON ug.user_id = s.user_id
INNER JOIN groups g ON g.id = ug.group_id
INNEr JOIN task_sheet ts ON ts.task_id = s.task_id
WHERE
  ($1 = 0 or s.user_id = $1)
AND
  ($2 = 0 or s.task_id = $2)
AND
  ($3 = 0 or ug.group_id = $3)
AND
  ($4 = 0 or ts.sheet_id = $4)
AND
  ($5 = 0 or g.course_id = $5)
`,
		filterUserID, filterTaskID, filterGroupID, filterSheetID, filterCourseID)
	return p, err
}
