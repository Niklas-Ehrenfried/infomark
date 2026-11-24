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

package app

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/franela/goblin"
	"github.com/infomark-org/infomark/email"
)

func TestTeam(t *testing.T) {
	g := goblin.Goblin(t)
	email.DefaultMail = email.VoidMail

	tape := NewTape()

	var stores *Stores

	studentAJWT := tape.NewJWTRequest(112, false)
	studentBJWT := tape.NewJWTRequest(113, false)
	tutorJWT := tape.NewJWTRequest(2, false)
	adminJWT := tape.NewJWTRequest(1, true)

	g.Describe("Teams", func() {

		g.BeforeEach(func() {
			tape.BeforeEach()
			stores = NewStores(tape.DB)
		})

		g.It("Query /team should require access claims", func() {
			w := tape.Get("/api/v1/courses/1/team")
			g.Assert(w.Code).Equal(http.StatusUnauthorized)
		})

		g.It("Student can view their team", func() {
			_, err := tape.DB.Exec(`UPDATE courses SET teams_enabled = true, team_formation_open = true, max_team_size = 2 WHERE id = 1;`)
			g.Assert(err).Equal(nil)

			w := tape.Get("/api/v1/courses/1/team", studentAJWT)
			g.Assert(w.Code).Equal(http.StatusOK)

			var resp TeamResponse
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			g.Assert(err).Equal(nil)
			g.Assert(len(resp.Members) >= 1).Equal(true)

			// Staff should be rejected from personal /team endpoint
			wStaff := tape.Get("/api/v1/courses/1/team", tutorJWT)
			g.Assert(wStaff.Code).Equal(http.StatusBadRequest)
		})

		g.It("GenerateTokenHandler requires team formation open", func() {
			_, err := tape.DB.Exec(`UPDATE courses SET teams_enabled = true, team_formation_open = false WHERE id = 1;`)
			g.Assert(err).Equal(nil)

			w := tape.Post("/api/v1/courses/1/team/token", H{}, studentAJWT)
			g.Assert(w.Code).Equal(http.StatusBadRequest)
		})

		g.It("Student can generate invite token when formation is open", func() {
			_, err := tape.DB.Exec(`UPDATE courses SET teams_enabled = true, team_formation_open = true, max_team_size = 2 WHERE id = 1;`)
			g.Assert(err).Equal(nil)

			w := tape.Post("/api/v1/courses/1/team/token", H{}, studentAJWT)
			g.Assert(w.Code).Equal(http.StatusOK)

			var tokenResp TeamTokenResponse
			err = json.Unmarshal(w.Body.Bytes(), &tokenResp)
			g.Assert(err).Equal(nil)
			g.Assert(tokenResp.Token != "").Equal(true)
		})

		g.It("Join team enforces GDPR consent and exercise group boundaries", func() {
			_, err := tape.DB.Exec(`UPDATE courses SET teams_enabled = true, team_formation_open = true, max_team_size = 2, enforce_same_exercise_group = true WHERE id = 1;`)
			g.Assert(err).Equal(nil)

			// Put student 112 in group 2 and student 113 in group 3 first to verify boundary rejection
			err = stores.Group.AssignStudentToGroup(112, 2, 1)
			g.Assert(err).Equal(nil)
			err = stores.Group.AssignStudentToGroup(113, 3, 1)
			g.Assert(err).Equal(nil)

			w1 := tape.Post("/api/v1/courses/1/team/token", H{}, studentAJWT)
			g.Assert(w1.Code).Equal(http.StatusOK)
			var tokenResp TeamTokenResponse
			_ = json.Unmarshal(w1.Body.Bytes(), &tokenResp)

			// Student B in different group is rejected
			wDiffGroup := tape.Post("/api/v1/courses/1/team/join", H{
				"token":   tokenResp.Token,
				"consent": true,
			}, studentBJWT)
			g.Assert(wDiffGroup.Code).Equal(http.StatusBadRequest)

			// Move Student B to the same exercise group (group 2)
			err = stores.Group.AssignStudentToGroup(113, 2, 1)
			g.Assert(err).Equal(nil)

			// Student B tries to join without consent
			w2 := tape.Post("/api/v1/courses/1/team/join", H{
				"token":   tokenResp.Token,
				"consent": false,
			}, studentBJWT)
			g.Assert(w2.Code).Equal(http.StatusBadRequest)

			// Student B joins with consent
			w3 := tape.Post("/api/v1/courses/1/team/join", H{
				"token":   tokenResp.Token,
				"consent": true,
			}, studentBJWT)
			g.Assert(w3.Code).Equal(http.StatusOK)
		})

		g.It("Roster management enforces role and tutor group scoping", func() {
			err := stores.Group.AssignStudentToGroup(112, 2, 1) // Group 2 has tutor_id = 2
			g.Assert(err).Equal(nil)
			err = stores.Group.AssignStudentToGroup(113, 3, 1) // Group 3 belongs to another tutor
			g.Assert(err).Equal(nil)

			// Ensure student 113 has a solo team in course 2 to test cross-course MoveMember IDOR
			otherCourseTeam, err := stores.Team.CreateSoloTeam(113, 2)
			g.Assert(err).Equal(nil)

			w := tape.Get("/api/v1/courses/1/teams", studentAJWT)
			g.Assert(w.Code).Equal(http.StatusForbidden)

			wTutor := tape.Get("/api/v1/courses/1/teams", tutorJWT)
			g.Assert(wTutor.Code).Equal(http.StatusOK)
			var rosterResp TeamRosterListResponse
			err = json.Unmarshal(wTutor.Body.Bytes(), &rosterResp)
			g.Assert(err).Equal(nil)
			// Every student returned to tutor 2 must belong to group 2
			for _, m := range rosterResp.Roster {
				g.Assert(m.GroupID.Valid).Equal(true)
				g.Assert(m.GroupID.Int64).Equal(int64(2))
			}

			// Admin cannot move student 112 into a team from course 2
			wCrossCourse := tape.Post("/api/v1/courses/1/teams/move", H{
				"user_id":        112,
				"target_team_id": otherCourseTeam.ID,
			}, adminJWT)
			g.Assert(wCrossCourse.Code).Equal(http.StatusBadRequest)

			// Tutor 2 cannot move student 112 into student 113's team in group 3
			group3Team, err := stores.Team.CreateSoloTeam(113, 1)
			g.Assert(err).Equal(nil)
			wCrossGroup := tape.Post("/api/v1/courses/1/teams/move", H{
				"user_id":        112,
				"target_team_id": group3Team.ID,
			}, tutorJWT)
			g.Assert(wCrossGroup.Code).Equal(http.StatusForbidden)
		})

		g.It("Admin can export grade summary as CSV", func() {
			wStudent := tape.Get("/api/v1/courses/1/grades/summary.csv", studentAJWT)
			g.Assert(wStudent.Code).Equal(http.StatusForbidden)

			wAdmin := tape.Get("/api/v1/courses/1/grades/summary.csv", adminJWT)
			g.Assert(wAdmin.Code).Equal(http.StatusOK)
			g.Assert(wAdmin.Header().Get("Content-Type")).Equal("text/csv; charset=utf-8")
		})

		g.AfterEach(func() {
			tape.AfterEach()
		})

	})
}
