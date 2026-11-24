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
	"net/http"
	"time"

	null "github.com/guregu/null/v6"
	"github.com/infomark-org/infomark/model"
)

// TeamResponse is the response payload for a student's own team view.
type TeamResponse struct {
	ID          int64                  `json:"id" example:"42"`
	TeamNumber  int64                  `json:"team_number" example:"1"`
	CourseID    int64                  `json:"course_id" example:"1"`
	InviteToken null.String            `json:"invite_token" example:"abcdef1234567890abcdef1234567890"`
	Members     []model.TeamMemberView `json:"members"`
	CreatedAt   time.Time              `json:"created_at"`
}

func (body *TeamResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func newTeamResponse(team *model.Team, members []model.TeamMemberView, teamNumber int64) *TeamResponse {
	return &TeamResponse{
		ID:          team.ID,
		TeamNumber:  teamNumber,
		CourseID:    team.CourseID,
		InviteToken: team.InviteToken,
		Members:     members,
		CreatedAt:   team.CreatedAt,
	}
}

// TeamTokenResponse is returned after generating an invite token.
type TeamTokenResponse struct {
	Token string `json:"token"`
}

func (body *TeamTokenResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func newTeamTokenResponse(token string) *TeamTokenResponse {
	return &TeamTokenResponse{Token: token}
}

// TeamRosterListResponse wraps the roster table data.
type TeamRosterListResponse struct {
	Roster []model.TeamRosterMember `json:"roster"`
}

func (body *TeamRosterListResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func newTeamRosterListResponse(roster []model.TeamRosterMember) *TeamRosterListResponse {
	return &TeamRosterListResponse{Roster: roster}
}
