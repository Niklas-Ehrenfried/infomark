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

package app

import (
	"errors"
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// GroupRequest is the request payload for course management.
type GroupRequest struct {
	// note, we will only use the id
	Tutor *struct {
		ID int64 `json:"id" example:"1"`
	} `json:"tutor"`
	TutorID     *int64 `json:"tutor_id"`
	Description string `json:"description" example:"Gruppe fuer ersties am Montag im Raum C25435"`
}

// Bind preprocesses a GroupRequest.
func (body *GroupRequest) Bind(r *http.Request) error {

	if body == nil {
		return errors.New("missing \"group\" data")
	}

	if body.Tutor == nil && body.TutorID != nil {
		body.Tutor = &struct {
			ID int64 `json:"id" example:"1"`
		}{ID: *body.TutorID}
	}

	if body.Tutor == nil {
		return errors.New("missing \"tutor\" data")
	}

	return body.Validate()

}

func (body *GroupRequest) Validate() error {

	err := validation.ValidateStruct(body,
		validation.Field(
			&body.Description,
			validation.Required,
		),
	)
	if err != nil {
		return err
	}

	return validation.ValidateStruct(body.Tutor,
		validation.Field(
			&body.Tutor.ID,
			validation.Required,
		),
	)
}

type GroupBidRequest struct {
	Bid int `json:"bid" example:"5" minval:"0" maxval:"10"`
}

// Bind preprocesses a GroupBidRequest.
func (body *GroupBidRequest) Bind(r *http.Request) error {
	return body.Validate()
}

func (body *GroupBidRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(
			&body.Bid,
			validation.Min(0),
			validation.Max(10),
		),
	)
}

// GroupRequest is the request payload for course management.
type GroupEnrollmentRequest struct {
	UserID int64 `json:"user_id" example:"15"`
}

// Bind preprocesses a GroupRequest.
func (body *GroupEnrollmentRequest) Bind(r *http.Request) error {
	if body == nil {
		return errors.New("missing \"group\" data")
	}
	return body.Validate()
}

func (body *GroupEnrollmentRequest) Validate() error {
	return validation.ValidateStruct(body,
		validation.Field(
			&body.UserID,
			validation.Required,
		),
	)
}

// AutoAssignRequest is payload for choosing a solver during auto-assignment.
type AutoAssignRequest struct {
	Solver string `json:"solver"`
}

func (body *AutoAssignRequest) Bind(r *http.Request) error {
	return nil
}

// GroupBidItem represents a single group preference bid.
type GroupBidItem struct {
	GroupID int64 `json:"group_id"`
	Bid     int   `json:"bid"`
}

// BatchGroupBidsRequest is payload for submitting multiple group preferences at once.
type BatchGroupBidsRequest struct {
	Bids []GroupBidItem `json:"bids"`
}

func (body *BatchGroupBidsRequest) Bind(r *http.Request) error {
	if body == nil || len(body.Bids) == 0 {
		return errors.New("missing bids data")
	}
	for _, b := range body.Bids {
		if b.GroupID <= 0 {
			return errors.New("invalid group_id")
		}
		if b.Bid < 0 || b.Bid > 10 {
			return errors.New("bid must be between 0 and 10")
		}
	}
	return nil
}
