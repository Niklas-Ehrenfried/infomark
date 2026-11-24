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
	"github.com/infomark-org/infomark/model"
)

func TestGroupSolvers(t *testing.T) {
	g := goblin.Goblin(t)
	email.DefaultMail = email.VoidMail

	tape := NewTape()
	var stores *Stores

	studentJWT := tape.NewJWTRequest(112, false)
	tutorJWT := tape.NewJWTRequest(2, false)
	adminJWT := tape.NewJWTRequest(1, true)

	g.Describe("GroupSolvers", func() {
		g.BeforeEach(func() {
			tape.BeforeEach()
			stores = NewStores(tape.DB)
		})

		g.It("Legacy single-bid endpoint allows bid 0 (cannot attend)", func() {
			w := tape.Post("/api/v1/courses/1/groups/1/bids", H{"bid": 0}, studentJWT)
			g.Assert(w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusNoContent).Equal(true)

			bid, err := stores.Group.GetBidOfUserForGroup(studentJWT.Claims.LoginID, 1)
			g.Assert(err).Equal(nil)
			g.Assert(bid).Equal(0)
		})

		g.It("Batch bids endpoint enforces anti-gaming minimum positive preferences", func() {
			_, err := tape.DB.Exec(`UPDATE courses SET min_group_preferences = 3 WHERE id = 1;`)
			g.Assert(err).Equal(nil)

			// Bullet-vote: only 1 positive bid (must reject with 400)
			wBad := tape.Post("/api/v1/courses/1/groups/bids", H{
				"bids": []H{
					{"group_id": 1, "bid": 10},
					{"group_id": 2, "bid": 0},
					{"group_id": 3, "bid": 0},
				},
			}, studentJWT)
			g.Assert(wBad.Code).Equal(http.StatusBadRequest)

			// Valid: 3 positive bids (must succeed)
			wGood := tape.Post("/api/v1/courses/1/groups/bids", H{
				"bids": []H{
					{"group_id": 1, "bid": 10},
					{"group_id": 2, "bid": 7},
					{"group_id": 3, "bid": 5},
				},
			}, studentJWT)
			g.Assert(wGood.Code).Equal(http.StatusOK)
		})

		g.It("Auto-assign and reset-assignments require admin role and run all 3 solvers", func() {
			// Students and tutors are forbidden
			wStu := tape.Post("/api/v1/courses/1/groups/auto-assign", H{"solver": "optimal"}, studentJWT)
			g.Assert(wStu.Code).Equal(http.StatusForbidden)
			wTut := tape.Post("/api/v1/courses/1/groups/auto-assign", H{"solver": "optimal"}, tutorJWT)
			g.Assert(wTut.Code).Equal(http.StatusForbidden)
			wResetTut := tape.Post("/api/v1/courses/1/groups/reset-assignments", H{}, tutorJWT)
			g.Assert(wResetTut.Code).Equal(http.StatusForbidden)

			for _, solver := range []string{"optimal", "regret", "greedy"} {
				w := tape.Post("/api/v1/courses/1/groups/auto-assign", H{"solver": solver}, adminJWT)
				g.Assert(w.Code).Equal(http.StatusOK)

				var res model.AutoAssignResult
				err := json.Unmarshal(w.Body.Bytes(), &res)
				g.Assert(err).Equal(nil)
				g.Assert(res.SolverUsed).Equal(solver)
				g.Assert(res.AssignedCount > 0).Equal(true)
			}

			wReset := tape.Post("/api/v1/courses/1/groups/reset-assignments", H{}, adminJWT)
			g.Assert(wReset.Code).Equal(http.StatusOK)
		})

		g.AfterEach(func() {
			tape.AfterEach()
		})
	})
}
