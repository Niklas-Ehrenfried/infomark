// InfoMark - a platform for managing courses with
//            distributing exercise sheets and testing exercise submissions
// Copyright (C) 2019-present InfoMark.org
// Authors: InfoMark Contributors
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
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/infomark-org/infomark/model"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type TeamStore struct {
	db *sqlx.DB
}

func NewTeamStore(db *sqlx.DB) *TeamStore {
	return &TeamStore{
		db: db,
	}
}

func (s *TeamStore) Get(teamID int64) (*model.Team, error) {
	p := model.Team{ID: teamID}
	err := s.db.Get(&p, "SELECT * FROM teams WHERE id = $1 LIMIT 1;", p.ID)
	return &p, err
}

func (s *TeamStore) GetByToken(token string) (*model.Team, error) {
	p := model.Team{}
	err := s.db.Get(&p, "SELECT * FROM teams WHERE invite_token = $1 LIMIT 1;", token)
	return &p, err
}

func (s *TeamStore) GetForUser(userID, courseID int64) (*model.Team, error) {
	p := model.Team{}
	err := s.db.Get(&p, `
SELECT
  t.*
FROM
  teams t
INNER JOIN
  team_members tm ON t.id = tm.team_id
WHERE
  tm.user_id = $1
AND
  tm.course_id = $2
LIMIT 1;`, userID, courseID)
	return &p, err
}

func (s *TeamStore) GetTeamNumber(teamID, courseID int64) int64 {
	var num int64
	_ = s.db.Get(&num, `
SELECT COALESCE((
  SELECT rank FROM (
    SELECT id, DENSE_RANK() OVER (ORDER BY id ASC) as rank
    FROM teams WHERE course_id = $1
  ) ranked WHERE id = $2
), 1);`, courseID, teamID)
	return num
}

func (s *TeamStore) GetMembers(teamID int64) ([]model.TeamMemberView, error) {
	members := []model.TeamMemberView{}
	err := s.db.Select(&members, `
SELECT
  u.id AS user_id,
  u.first_name,
  u.last_name,
  u.avatar_url,
  tm.joined_at
FROM
  team_members tm
INNER JOIN
  users u ON tm.user_id = u.id
WHERE
  tm.team_id = $1
ORDER BY
  tm.joined_at ASC;`, teamID)
	return members, err
}

func (s *TeamStore) GetMembersFull(teamID int64) ([]model.TeamMember, error) {
	members := []model.TeamMember{}
	err := s.db.Select(&members, `SELECT * FROM team_members WHERE team_id = $1 ORDER BY joined_at ASC;`, teamID)
	return members, err
}

func (s *TeamStore) CreateSoloTeam(userID, courseID int64) (*model.Team, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// If another request already initialized a team for this user, reuse it cleanly.
	var existingTeamID int64
	if err := tx.Get(&existingTeamID, `SELECT team_id FROM team_members WHERE user_id = $1 AND course_id = $2 LIMIT 1;`, userID, courseID); err == nil && existingTeamID > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return s.Get(existingTeamID)
	}

	team := &model.Team{CourseID: courseID}
	newID, err := Insert(tx, "teams", team)
	if err != nil {
		return nil, err
	}

	var insertedTeamID int64
	err = tx.Get(&insertedTeamID, `
INSERT INTO team_members (team_id, user_id, course_id, joined_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, course_id) DO NOTHING
RETURNING team_id;`, newID, userID, courseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Concurrent transaction inserted first; discard the unused team row.
			_ = tx.Rollback()
			return s.GetForUser(userID, courseID)
		}
		return nil, err
	}

	// Associate only open team-sheet submissions in this course that don't have a team_id yet.
	_, err = tx.Exec(`
UPDATE submissions s
SET team_id = $1
FROM task_sheet ts
INNER JOIN sheet_course sc ON ts.sheet_id = sc.sheet_id
INNER JOIN sheets sh ON ts.sheet_id = sh.id
WHERE s.task_id = ts.task_id
  AND sc.course_id = $2
  AND sh.allow_teams = true
  AND s.user_id = $3
  AND s.team_id IS NULL;`, newID, courseID, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.Get(newID)
}

func (s *TeamStore) EnsureSoloTeams(courseID int64) error {
	var userIDs []int64
	err := s.db.Select(&userIDs, `
SELECT uc.user_id
FROM user_course uc
LEFT JOIN team_members tm ON uc.user_id = tm.user_id AND tm.course_id = uc.course_id
WHERE uc.course_id = $1
  AND uc.role = 0
  AND tm.id IS NULL;`, courseID)
	if err != nil {
		return err
	}

	for _, uid := range userIDs {
		if _, err := s.CreateSoloTeam(uid, courseID); err != nil {
			return err
		}
	}
	return nil
}

func (s *TeamStore) EnsureSingleGroupEnrollment(courseID int64) error {
	var groupCount int
	err := s.db.Get(&groupCount, `SELECT COUNT(*) FROM groups WHERE course_id = $1;`, courseID)
	if err != nil || groupCount != 1 {
		return err
	}

	var groupID int64
	err = s.db.Get(&groupID, `SELECT id FROM groups WHERE course_id = $1 LIMIT 1;`, courseID)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
INSERT INTO user_group (user_id, group_id)
SELECT uc.user_id, $1
FROM user_course uc
LEFT JOIN user_group ug ON uc.user_id = ug.user_id AND ug.group_id = $1
WHERE uc.course_id = $2
  AND uc.role = 0
  AND ug.id IS NULL;`, groupID, courseID)
	return err
}

func (s *TeamStore) GenerateToken(teamID int64) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := fmt.Sprintf("%x", b)

	_, err := s.db.Exec(`UPDATE teams SET invite_token = $1, updated_at = NOW() WHERE id = $2;`, token, teamID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *TeamStore) ClearTokensForCourse(courseID int64) error {
	_, err := s.db.Exec(`UPDATE teams SET invite_token = NULL, updated_at = NOW() WHERE course_id = $1;`, courseID)
	return err
}

func (s *TeamStore) DeleteMember(userID, courseID int64) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var teamID int64
	err = tx.Get(&teamID, `SELECT team_id FROM team_members WHERE user_id = $1 AND course_id = $2 LIMIT 1;`, userID, courseID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	if _, err := tx.Exec(`DELETE FROM team_members WHERE user_id = $1 AND course_id = $2;`, userID, courseID); err != nil {
		return err
	}

	var remaining int
	if err := tx.Get(&remaining, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, teamID); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err := tx.Exec(`DELETE FROM teams WHERE id = $1;`, teamID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// JoinTeam handles Student B entering Student A's join token.
// Returns list of [srcSubmissionID, dstSubmissionID] pairs whose files need syncing.
func (s *TeamStore) JoinTeam(userBID, targetTeamID, courseID int64, consent bool) ([][2]int64, error) {
	if !consent {
		return nil, errors.New("consent is required to join a team")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Verify course settings
	var course model.Course
	err = tx.Get(&course, `SELECT * FROM courses WHERE id = $1 LIMIT 1;`, courseID)
	if err != nil {
		return nil, err
	}
	if !course.TeamsEnabled {
		return nil, errors.New("teams are not enabled for this course")
	}
	if !course.TeamFormationOpen {
		return nil, errors.New("team formation is currently closed")
	}

	// Look up User B's current team first so both teams can be locked in ascending ID order.
	var userBTeamID int64
	err = tx.Get(&userBTeamID, `SELECT team_id FROM team_members WHERE user_id = $1 AND course_id = $2 LIMIT 1;`, userBID, courseID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if userBTeamID == targetTeamID {
		return nil, errors.New("you are already a member of this team")
	}

	// Lock involved teams in deterministic order to avoid concurrent join races and deadlocks.
	if userBTeamID > 0 {
		var lockedIDs []int64
		if err := tx.Select(&lockedIDs, `SELECT id FROM teams WHERE id IN ($1, $2) AND course_id = $3 ORDER BY id ASC FOR UPDATE;`, targetTeamID, userBTeamID, courseID); err != nil {
			return nil, err
		}
	}

	// 2. Verify target team
	var targetTeam model.Team
	err = tx.Get(&targetTeam, `SELECT * FROM teams WHERE id = $1 AND course_id = $2 LIMIT 1 FOR UPDATE;`, targetTeamID, courseID)
	if err != nil {
		return nil, errors.New("target team not found")
	}

	var memberCount int
	if err := tx.Get(&memberCount, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, targetTeamID); err != nil {
		return nil, err
	}
	if memberCount >= course.MaxTeamSize {
		return nil, errors.New("target team is already full")
	}

	// 3. Re-verify User B is still in a solo team (strict lock-in)
	var userBMemberCount int
	if userBTeamID > 0 {
		if err := tx.Get(&userBMemberCount, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, userBTeamID); err != nil {
			return nil, err
		}
		if userBMemberCount > 1 {
			return nil, errors.New("students cannot leave or change teams once joined with a partner")
		}
	}

	// 4. Exercise group boundary check
	var totalGroups int
	if err := tx.Get(&totalGroups, `SELECT COUNT(*) FROM groups WHERE course_id = $1;`, courseID); err != nil {
		return nil, err
	}

	if totalGroups == 1 {
		// Auto-enroll both if not yet enrolled
		var groupID int64
		if err := tx.Get(&groupID, `SELECT id FROM groups WHERE course_id = $1 LIMIT 1;`, courseID); err == nil {
			if _, err := tx.Exec(`
INSERT INTO user_group (user_id, group_id)
SELECT $1, $2
WHERE NOT EXISTS (SELECT 1 FROM user_group WHERE user_id = $1 AND group_id = $2);`, userBID, groupID); err != nil {
				return nil, err
			}
		}
	} else if totalGroups >= 2 && course.EnforceSameExerciseGroup {
		// Joining student must belong to an exercise group
		var userBGroupID int64
		err = tx.Get(&userBGroupID, `
SELECT ug.group_id
FROM user_group ug
INNER JOIN groups g ON ug.group_id = g.id
WHERE ug.user_id = $1 AND g.course_id = $2 LIMIT 1;`, userBID, courseID)
		if err != nil {
			return nil, errors.New("you must be enrolled in an exercise group before joining a team")
		}

		// Every target team member must be in an exercise group matching userBGroupID
		var targetMemberGroupIDs []int64
		err = tx.Select(&targetMemberGroupIDs, `
SELECT g.id
FROM team_members tm
INNER JOIN user_group ug ON tm.user_id = ug.user_id
INNER JOIN groups g ON ug.group_id = g.id AND g.course_id = tm.course_id
WHERE tm.team_id = $1;`, targetTeamID)
		if err != nil {
			return nil, err
		}

		if len(targetMemberGroupIDs) < memberCount {
			return nil, errors.New("all team members must be enrolled in the same exercise group")
		}
		for _, gid := range targetMemberGroupIDs {
			if gid != userBGroupID {
				return nil, errors.New("all team members must be enrolled in the same exercise group")
			}
		}
	}

	// 5. Delete user B's old solo team
	if userBTeamID > 0 {
		if _, err := tx.Exec(`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;`, userBTeamID, userBID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`DELETE FROM teams WHERE id = $1;`, userBTeamID); err != nil {
			return nil, err
		}
	}

	// 6. Add user B to target team
	_, err = tx.Exec(`
INSERT INTO team_members (team_id, user_id, course_id, joined_at, consented_at)
VALUES ($1, $2, $3, NOW(), NOW());`, targetTeamID, userBID, courseID)
	if err != nil {
		return nil, err
	}

	// 7. Sync open submissions
	syncedSubmissions, err := s.syncOpenSubmissionsForUser(tx, userBID, targetTeamID, courseID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return syncedSubmissions, nil
}

// MoveMember relocates a student to another team (tutor/admin action).
func (s *TeamStore) MoveMember(userID, targetTeamID, courseID int64) ([][2]int64, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var validTargetID int64
	if err := tx.Get(&validTargetID, `SELECT id FROM teams WHERE id = $1 AND course_id = $2 LIMIT 1 FOR UPDATE;`, targetTeamID, courseID); err != nil {
		return nil, errors.New("target team not found in this course")
	}

	var oldTeamID int64
	err = tx.Get(&oldTeamID, `SELECT team_id FROM team_members WHERE user_id = $1 AND course_id = $2 LIMIT 1;`, userID, courseID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if oldTeamID == targetTeamID {
		return nil, nil
	}

	if oldTeamID > 0 {
		if _, err := tx.Exec(`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;`, oldTeamID, userID); err != nil {
			return nil, err
		}
		var remaining int
		if err := tx.Get(&remaining, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, oldTeamID); err == nil && remaining == 0 {
			if _, err := tx.Exec(`DELETE FROM teams WHERE id = $1;`, oldTeamID); err != nil {
				return nil, err
			}
		}
	}

	_, err = tx.Exec(`
INSERT INTO team_members (team_id, user_id, course_id, joined_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, course_id) DO UPDATE SET team_id = EXCLUDED.team_id, joined_at = NOW();`, targetTeamID, userID, courseID)
	if err != nil {
		return nil, err
	}

	syncedSubmissions, err := s.syncOpenSubmissionsForUser(tx, userID, targetTeamID, courseID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return syncedSubmissions, nil
}

// SplitToSolo removes a student from their team and gives them a new solo team (tutor/admin action).
func (s *TeamStore) SplitToSolo(userID, courseID int64) (*model.Team, error) {
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var oldTeamID int64
	err = tx.Get(&oldTeamID, `SELECT team_id FROM team_members WHERE user_id = $1 AND course_id = $2 LIMIT 1;`, userID, courseID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var memberCount int
	if oldTeamID > 0 {
		_ = tx.Get(&memberCount, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, oldTeamID)
		if memberCount <= 1 {
			// Already solo
			team := &model.Team{ID: oldTeamID, CourseID: courseID}
			return team, nil
		}
		if _, err := tx.Exec(`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2;`, oldTeamID, userID); err != nil {
			return nil, err
		}
	}

	// Create new solo team
	newTeam := &model.Team{CourseID: courseID}
	newID, err := Insert(tx, "teams", newTeam)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
INSERT INTO team_members (team_id, user_id, course_id, joined_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, course_id) DO UPDATE SET team_id = EXCLUDED.team_id;`, newID, userID, courseID)
	if err != nil {
		return nil, err
	}

	// Re-tag user's open submissions to the new solo team
	if _, err := tx.Exec(`
UPDATE submissions s
SET team_id = $1
FROM task_sheet ts
INNER JOIN sheet_course sc ON ts.sheet_id = sc.sheet_id
INNER JOIN sheets sh ON ts.sheet_id = sh.id
WHERE s.task_id = ts.task_id
  AND sc.course_id = $2
  AND sh.allow_teams = true
  AND sh.due_at > NOW()
  AND s.user_id = $3;`, newID, courseID, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.Get(newID)
}

// CreateCustomTeam groups an arbitrary set of enrolled students directly (tutor/admin action).
func (s *TeamStore) CreateCustomTeam(userIDs []int64, courseID int64) (*model.Team, [][2]int64, error) {
	seen := make(map[int64]bool, len(userIDs))
	uniqueIDs := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 && !seen[uid] {
			seen[uid] = true
			uniqueIDs = append(uniqueIDs, uid)
		}
	}
	if len(uniqueIDs) == 0 {
		return nil, nil, errors.New("at least one valid student must be specified")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Verify all specified users are enrolled as students in this course.
	var enrolledCount int
	if err := tx.Get(&enrolledCount, `
SELECT COUNT(*)
FROM user_course
WHERE course_id = $1 AND role = 0 AND user_id = ANY($2);`, courseID, pq.Array(uniqueIDs)); err != nil {
		return nil, nil, err
	}
	if enrolledCount != len(uniqueIDs) {
		return nil, nil, errors.New("all team members must be enrolled students in this course")
	}

	team := &model.Team{CourseID: courseID}
	newID, err := Insert(tx, "teams", team)
	if err != nil {
		return nil, nil, err
	}

	var oldTeamIDs []int64
	err = tx.Select(&oldTeamIDs, `
SELECT DISTINCT team_id
FROM team_members
WHERE user_id = ANY($1) AND course_id = $2;`, pq.Array(uniqueIDs), courseID)
	if err != nil {
		return nil, nil, err
	}

	for _, uid := range uniqueIDs {
		if _, err := tx.Exec(`DELETE FROM team_members WHERE user_id = $1 AND course_id = $2;`, uid, courseID); err != nil {
			return nil, nil, err
		}
		_, err = tx.Exec(`
INSERT INTO team_members (team_id, user_id, course_id, joined_at)
VALUES ($1, $2, $3, NOW());`, newID, uid, courseID)
		if err != nil {
			return nil, nil, err
		}
	}

	// Sync open submissions across all grouped members
	var allSynced [][2]int64
	for _, uid := range uniqueIDs {
		synced, err := s.syncOpenSubmissionsForUser(tx, uid, newID, courseID)
		if err != nil {
			return nil, nil, err
		}
		allSynced = append(allSynced, synced...)
	}

	// Cleanup old teams that are now empty
	for _, otid := range oldTeamIDs {
		var cnt int
		if err := tx.Get(&cnt, `SELECT COUNT(*) FROM team_members WHERE team_id = $1;`, otid); err == nil && cnt == 0 {
			if _, err := tx.Exec(`DELETE FROM teams WHERE id = $1;`, otid); err != nil {
				return nil, nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	createdTeam, err := s.Get(newID)
	return createdTeam, allSynced, err
}

// syncOpenSubmissionsForUser copies open-sheet submissions between the user and target team members.
func (s *TeamStore) syncOpenSubmissionsForUser(tx *sqlx.Tx, userID, targetTeamID, courseID int64) ([][2]int64, error) {
	var synced [][2]int64

	type OpenTask struct {
		TaskID  int64 `db:"task_id"`
		SheetID int64 `db:"sheet_id"`
	}
	var openTasks []OpenTask
	err := tx.Select(&openTasks, `
SELECT ts.task_id, ts.sheet_id
FROM tasks t
INNER JOIN task_sheet ts ON t.id = ts.task_id
INNER JOIN sheet_course sc ON ts.sheet_id = sc.sheet_id
INNER JOIN sheets sh ON ts.sheet_id = sh.id
WHERE sc.course_id = $1
  AND sh.allow_teams = true
  AND sh.due_at > NOW();`, courseID)
	if err != nil {
		return nil, err
	}

	for _, ot := range openTasks {
		// 1. Does target team already have a submission?
		type SubGrade struct {
			SubmissionID          int64         `db:"sub_id"`
			SubmittedByUserID     sql.NullInt64 `db:"submitted_by_user_id"`
			IsFinal               bool          `db:"is_final"`
			PublicExecutionState  int           `db:"public_execution_state"`
			PrivateExecutionState int           `db:"private_execution_state"`
			PublicTestLog         string        `db:"public_test_log"`
			PrivateTestLog        string        `db:"private_test_log"`
			PublicTestStatus      int           `db:"public_test_status"`
			PrivateTestStatus     int           `db:"private_test_status"`
			AcquiredPoints        int           `db:"acquired_points"`
			Feedback              string        `db:"feedback"`
			TutorID               int64         `db:"tutor_id"`
		}

		var teamSub SubGrade
		teamErr := tx.Get(&teamSub, `
SELECT s.id AS sub_id, s.submitted_by_user_id, s.is_final,
       g.public_execution_state, g.private_execution_state, g.public_test_log, g.private_test_log,
       g.public_test_status, g.private_test_status, g.acquired_points, g.feedback, g.tutor_id
FROM submissions s
INNER JOIN grades g ON g.submission_id = s.id
WHERE s.team_id = $1 AND s.task_id = $2
ORDER BY s.id DESC LIMIT 1;`, targetTeamID, ot.TaskID)

		if teamErr == nil {
			// Target team has a submission. Copy to joining user.
			var userSubID int64
			userSubErr := tx.Get(&userSubID, `SELECT id FROM submissions WHERE user_id = $1 AND task_id = $2 LIMIT 1;`, userID, ot.TaskID)
			if userSubErr == nil {
				// Update user B submission
				_, err = tx.Exec(`
UPDATE submissions
SET team_id = $1, submitted_by_user_id = $2, is_final = $3, updated_at = NOW()
WHERE id = $4;`, targetTeamID, teamSub.SubmittedByUserID, teamSub.IsFinal, userSubID)
				if err != nil {
					return nil, err
				}

				// Update user B grade
				_, err = tx.Exec(`
UPDATE grades
SET public_execution_state = $1, private_execution_state = $2, public_test_log = $3, private_test_log = $4,
    public_test_status = $5, private_test_status = $6, acquired_points = $7, feedback = $8, tutor_id = $9,
    is_individual_override = false, updated_at = NOW()
WHERE submission_id = $10;`,
					teamSub.PublicExecutionState, teamSub.PrivateExecutionState, teamSub.PublicTestLog, teamSub.PrivateTestLog,
					teamSub.PublicTestStatus, teamSub.PrivateTestStatus, teamSub.AcquiredPoints, teamSub.Feedback, teamSub.TutorID,
					userSubID)
				if err != nil {
					return nil, err
				}

				synced = append(synced, [2]int64{teamSub.SubmissionID, userSubID})
			} else {
				// Insert user B submission
				err = tx.Get(&userSubID, `
INSERT INTO submissions (user_id, task_id, team_id, submitted_by_user_id, is_final)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;`, userID, ot.TaskID, targetTeamID, teamSub.SubmittedByUserID, teamSub.IsFinal)
				if err != nil {
					return nil, err
				}

				// Insert user B grade
				_, err = tx.Exec(`
INSERT INTO grades (public_execution_state, private_execution_state, public_test_log, private_test_log,
                    public_test_status, private_test_status, acquired_points, feedback, tutor_id,
                    submission_id, is_individual_override)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, false);`,
					teamSub.PublicExecutionState, teamSub.PrivateExecutionState, teamSub.PublicTestLog, teamSub.PrivateTestLog,
					teamSub.PublicTestStatus, teamSub.PrivateTestStatus, teamSub.AcquiredPoints, teamSub.Feedback, teamSub.TutorID,
					userSubID)
				if err != nil {
					return nil, err
				}

				synced = append(synced, [2]int64{teamSub.SubmissionID, userSubID})
			}
		} else {
			// Target team does not have a submission. Does user B have one?
			var userSub SubGrade
			uErr := tx.Get(&userSub, `
SELECT s.id AS sub_id, s.submitted_by_user_id, s.is_final,
       g.public_execution_state, g.private_execution_state, g.public_test_log, g.private_test_log,
       g.public_test_status, g.private_test_status, g.acquired_points, g.feedback, g.tutor_id
FROM submissions s
INNER JOIN grades g ON g.submission_id = s.id
WHERE s.user_id = $1 AND s.task_id = $2 LIMIT 1;`, userID, ot.TaskID)

			if uErr == nil {
				// Tag user B's submission with target team
				_, _ = tx.Exec(`UPDATE submissions SET team_id = $1 WHERE id = $2;`, targetTeamID, userSub.SubmissionID)

				// Copy to other members of target team
				var otherMemberIDs []int64
				_ = tx.Select(&otherMemberIDs, `SELECT user_id FROM team_members WHERE team_id = $1 AND user_id <> $2;`, targetTeamID, userID)

				for _, otherUID := range otherMemberIDs {
					var otherSubID int64
					oErr := tx.Get(&otherSubID, `SELECT id FROM submissions WHERE user_id = $1 AND task_id = $2 LIMIT 1;`, otherUID, ot.TaskID)
					if oErr == nil {
						_, _ = tx.Exec(`
UPDATE submissions
SET team_id = $1, submitted_by_user_id = $2, is_final = $3, updated_at = NOW()
WHERE id = $4;`, targetTeamID, userSub.SubmittedByUserID, userSub.IsFinal, otherSubID)

						_, _ = tx.Exec(`
UPDATE grades
SET public_execution_state = $1, private_execution_state = $2, public_test_log = $3, private_test_log = $4,
    public_test_status = $5, private_test_status = $6, acquired_points = $7, feedback = $8, tutor_id = $9,
    is_individual_override = false, updated_at = NOW()
WHERE submission_id = $10;`,
							userSub.PublicExecutionState, userSub.PrivateExecutionState, userSub.PublicTestLog, userSub.PrivateTestLog,
							userSub.PublicTestStatus, userSub.PrivateTestStatus, userSub.AcquiredPoints, userSub.Feedback, userSub.TutorID,
							otherSubID)

						synced = append(synced, [2]int64{userSub.SubmissionID, otherSubID})
					} else {
						err = tx.Get(&otherSubID, `
INSERT INTO submissions (user_id, task_id, team_id, submitted_by_user_id, is_final)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;`, otherUID, ot.TaskID, targetTeamID, userSub.SubmittedByUserID, userSub.IsFinal)
						if err == nil {
							_, _ = tx.Exec(`
INSERT INTO grades (public_execution_state, private_execution_state, public_test_log, private_test_log,
                    public_test_status, private_test_status, acquired_points, feedback, tutor_id,
                    submission_id, is_individual_override)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, false);`,
								userSub.PublicExecutionState, userSub.PrivateExecutionState, userSub.PublicTestLog, userSub.PrivateTestLog,
								userSub.PublicTestStatus, userSub.PrivateTestStatus, userSub.AcquiredPoints, userSub.Feedback, userSub.TutorID,
								otherSubID)

							synced = append(synced, [2]int64{userSub.SubmissionID, otherSubID})
						}
					}
				}
			}
		}
	}

	return synced, nil
}

// ListRoster returns the full team roster for admins, or scoped to the tutor's groups for tutors.
func (s *TeamStore) ListRoster(courseID int64, tutorID int64, query string) ([]model.TeamRosterMember, error) {
	roster := []model.TeamRosterMember{}

	baseQuery := `
WITH ranked_teams AS (
  SELECT id, DENSE_RANK() OVER (ORDER BY id ASC) AS team_number
  FROM teams
  WHERE course_id = $1
)
SELECT
  u.id AS user_id,
  u.first_name,
  u.last_name,
  u.email,
  u.student_number,
  u.avatar_url,
  g.id AS group_id,
  g.description AS group_description,
  COALESCE(tm.team_id, 0) AS team_id,
  COALESCE(rt.team_number, 0) AS team_number,
  COALESCE((SELECT COUNT(*) FROM team_members tm2 WHERE tm2.team_id = tm.team_id), 0) AS team_size,
  COALESCE(tm.joined_at, u.created_at) AS joined_at
FROM
  user_course uc
INNER JOIN
  users u ON uc.user_id = u.id
LEFT JOIN
  team_members tm ON uc.user_id = tm.user_id AND tm.course_id = uc.course_id
LEFT JOIN
  ranked_teams rt ON rt.id = tm.team_id
LEFT JOIN
  (user_group ug INNER JOIN groups g ON ug.group_id = g.id) ON u.id = ug.user_id AND g.course_id = uc.course_id
WHERE
  uc.course_id = $1
  AND uc.role = 0`

	args := []interface{}{courseID}
	argIdx := 2
	if tutorID > 0 {
		baseQuery += fmt.Sprintf(` AND g.tutor_id = $%d`, argIdx)
		args = append(args, tutorID)
		argIdx++
	}
	if strings.TrimSpace(query) != "" {
		baseQuery += fmt.Sprintf(`
  AND (
    LOWER(u.first_name) LIKE $%d
    OR LOWER(u.last_name) LIKE $%d
    OR LOWER(u.email) LIKE $%d
    OR LOWER(u.student_number) LIKE $%d
  )`, argIdx, argIdx, argIdx, argIdx)
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(query))+"%")
	}

	baseQuery += ` ORDER BY tm.team_id ASC, u.last_name ASC;`

	err := s.db.Select(&roster, baseQuery, args...)
	return roster, err
}
