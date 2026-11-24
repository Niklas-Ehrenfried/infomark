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
	"errors"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// JoinTeamRequest is the payload for Student B entering Student A's token.
type JoinTeamRequest struct {
	Token   string `json:"token" example:"abcdef1234567890abcdef1234567890"`
	Consent bool   `json:"consent" example:"true"`
}

func (body *JoinTeamRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing request body")
	}
	return body.Validate()
}

func (body *JoinTeamRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(&body.Token, validation.Required),
		validation.Field(&body.Consent, validation.Required),
	)
}

// MoveTeamMemberRequest relocates a student to target team.
type MoveTeamMemberRequest struct {
	UserID       int64 `json:"user_id" example:"42"`
	TargetTeamID int64 `json:"target_team_id" example:"7"`
}

func (body *MoveTeamMemberRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing request body")
	}
	return body.Validate()
}

func (body *MoveTeamMemberRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(&body.UserID, validation.Required),
		validation.Field(&body.TargetTeamID, validation.Required),
	)
}

// SplitTeamMemberRequest ejects a student into their own solo team.
type SplitTeamMemberRequest struct {
	UserID int64 `json:"user_id" example:"42"`
}

func (body *SplitTeamMemberRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing request body")
	}
	return body.Validate()
}

func (body *SplitTeamMemberRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(&body.UserID, validation.Required),
	)
}

// CreateCustomTeamRequest groups arbitrary students directly into a team.
type CreateCustomTeamRequest struct {
	UserIDs []int64 `json:"user_ids"`
}

func (body *CreateCustomTeamRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing request body")
	}
	return body.Validate()
}

func (body *CreateCustomTeamRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(&body.UserIDs, validation.Required),
	)
}
