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

	"github.com/go-chi/render"
	"github.com/infomark-org/infomark/api/helper"
	"github.com/infomark-org/infomark/auth/authenticate"
	"github.com/infomark-org/infomark/auth/authorize"
	"github.com/infomark-org/infomark/model"
	"github.com/infomark-org/infomark/symbol"
)

// TeamResource handles course-level team formation and management endpoints.
type TeamResource struct {
	Stores *Stores
}

func NewTeamResource(stores *Stores) *TeamResource {
	return &TeamResource{
		Stores: stores,
	}
}

// checkTutorAuthorization verifies that tutors only modify students in their own exercise groups.
// Admins can manage any student in the course.
func (rs *TeamResource) checkTutorAuthorization(tutorID, targetUserID, courseID int64, role authorize.CourseRole) error {
	if role >= authorize.ADMIN {
		return nil
	}

	tutorGroups, err := rs.Stores.Group.GetOfTutor(tutorID, courseID)
	if err != nil || len(tutorGroups) == 0 {
		return errors.New("you do not have any exercise groups in this course")
	}

	targetEnrollment, err := rs.Stores.Group.GetGroupEnrollmentOfUserInCourse(targetUserID, courseID)
	if err != nil {
		return errors.New("student is not assigned to your exercise group")
	}

	for _, tg := range tutorGroups {
		if tg.ID == targetEnrollment.GroupID {
			return nil
		}
	}
	return errors.New("you are only authorized to manage students in your own exercise group(s)")
}

// GetTeamHandler returns the current student's team and peer member details.
// URL: /courses/{course_id}/team
// METHOD: get
// TAG: teams
// RESPONSE: 200,TeamResponse
func (rs *TeamResource) GetTeamHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	if courseRole != authorize.STUDENT {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("only students can manage personal team membership")))
		return
	}

	team, err := rs.Stores.Team.GetForUser(accessClaims.LoginID, course.ID)
	if err != nil {
		if course.TeamsEnabled {
			// Auto-initialize solo team if teams are enabled
			team, err = rs.Stores.Team.CreateSoloTeam(accessClaims.LoginID, course.ID)
			if err != nil {
				_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
				return
			}
		} else {
			_ = render.Render(w, r, ErrNotFound)
			return
		}
	}

	members, err := rs.Stores.Team.GetMembers(team.ID)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	teamNumber := rs.Stores.Team.GetTeamNumber(team.ID, course.ID)
	if err := render.Render(w, r, newTeamResponse(team, members, teamNumber)); err != nil {
		_ = render.Render(w, r, ErrRender(err))
		return
	}
	render.Status(r, http.StatusOK)
}

// GenerateTokenHandler creates or regenerates an invite token for the team.
// URL: /courses/{course_id}/team/token
// METHOD: post
// TAG: teams
// RESPONSE: 200,TeamTokenResponse
func (rs *TeamResource) GenerateTokenHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	if courseRole != authorize.STUDENT {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("only students can generate team invite tokens")))
		return
	}

	if !course.TeamsEnabled {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("teams are not enabled for this course")))
		return
	}
	if !course.TeamFormationOpen {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("team formation is currently closed")))
		return
	}

	team, err := rs.Stores.Team.GetForUser(accessClaims.LoginID, course.ID)
	if err != nil {
		team, err = rs.Stores.Team.CreateSoloTeam(accessClaims.LoginID, course.ID)
		if err != nil {
			_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
			return
		}
	}

	members, err := rs.Stores.Team.GetMembers(team.ID)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}
	if len(members) >= course.MaxTeamSize {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("team has already reached the maximum team size")))
		return
	}

	token, err := rs.Stores.Team.GenerateToken(team.ID)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	if err := render.Render(w, r, newTeamTokenResponse(token)); err != nil {
		_ = render.Render(w, r, ErrRender(err))
		return
	}
	render.Status(r, http.StatusOK)
}

// JoinTeamHandler merges the calling student into a target team using an invite token.
// URL: /courses/{course_id}/team/join
// METHOD: post
// TAG: teams
// REQUEST: JoinTeamRequest
// RESPONSE: 200,TeamResponse
func (rs *TeamResource) JoinTeamHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	if courseRole != authorize.STUDENT {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("only students can join a team")))
		return
	}

	data := &JoinTeamRequest{}
	if err := render.Bind(r, data); err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	targetTeam, err := rs.Stores.Team.GetByToken(data.Token)
	if err != nil || targetTeam.CourseID != course.ID {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("invalid or expired invite token")))
		return
	}

	syncedPairs, err := rs.Stores.Team.JoinTeam(accessClaims.LoginID, targetTeam.ID, course.ID, data.Consent)
	if err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	// Copy submission files between synced submission records
	for _, pair := range syncedPairs {
		src := helper.NewSubmissionFileHandle(pair[0])
		dst := helper.NewSubmissionFileHandle(pair[1])
		_ = src.CopyTo(dst)
	}

	updatedTeam, err := rs.Stores.Team.Get(targetTeam.ID)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	members, err := rs.Stores.Team.GetMembers(targetTeam.ID)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	teamNumber := rs.Stores.Team.GetTeamNumber(updatedTeam.ID, course.ID)
	if err := render.Render(w, r, newTeamResponse(updatedTeam, members, teamNumber)); err != nil {
		_ = render.Render(w, r, ErrRender(err))
		return
	}
	render.Status(r, http.StatusOK)
}

// ListRosterHandler lists students, their groups, and teams in the course (scoped to own groups for tutors).
// URL: /courses/{course_id}/teams
// METHOD: get
// TAG: teams
// RESPONSE: 200,TeamRosterListResponse
func (rs *TeamResource) ListRosterHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)
	query := helper.StringFromURL(r, "q", "")

	tutorID := accessClaims.LoginID
	if courseRole >= authorize.ADMIN {
		tutorID = 0
	}

	roster, err := rs.Stores.Team.ListRoster(course.ID, tutorID, query)
	if err != nil {
		_ = render.Render(w, r, ErrInternalServerErrorWithDetails(err))
		return
	}

	if err := render.Render(w, r, newTeamRosterListResponse(roster)); err != nil {
		_ = render.Render(w, r, ErrRender(err))
		return
	}
	render.Status(r, http.StatusOK)
}

// MoveMemberHandler relocates a student to a target team.
// URL: /courses/{course_id}/teams/move
// METHOD: post
// TAG: teams
// REQUEST: MoveTeamMemberRequest
// RESPONSE: 204,NoContent
func (rs *TeamResource) MoveMemberHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	data := &MoveTeamMemberRequest{}
	if err := render.Bind(r, data); err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	if err := rs.checkTutorAuthorization(accessClaims.LoginID, data.UserID, course.ID, courseRole); err != nil {
		_ = render.Render(w, r, ErrUnauthorizedWithDetails(err))
		return
	}

	targetTeam, err := rs.Stores.Team.Get(data.TargetTeamID)
	if err != nil || targetTeam.CourseID != course.ID {
		_ = render.Render(w, r, ErrBadRequestWithDetails(errors.New("target team not found in this course")))
		return
	}

	if courseRole < authorize.ADMIN {
		targetMembers, err := rs.Stores.Team.GetMembers(targetTeam.ID)
		if err != nil || len(targetMembers) == 0 {
			_ = render.Render(w, r, ErrUnauthorizedWithDetails(errors.New("target team is not in your exercise group")))
			return
		}
		for _, m := range targetMembers {
			if err := rs.checkTutorAuthorization(accessClaims.LoginID, m.UserID, course.ID, courseRole); err != nil {
				_ = render.Render(w, r, ErrUnauthorizedWithDetails(errors.New("target team belongs to another tutor's exercise group")))
				return
			}
		}
	}

	syncedPairs, err := rs.Stores.Team.MoveMember(data.UserID, data.TargetTeamID, course.ID)
	if err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	for _, pair := range syncedPairs {
		src := helper.NewSubmissionFileHandle(pair[0])
		dst := helper.NewSubmissionFileHandle(pair[1])
		_ = src.CopyTo(dst)
	}

	render.Status(r, http.StatusNoContent)
}

// SplitMemberHandler ejects a student from their team back into a solo team.
// URL: /courses/{course_id}/teams/split
// METHOD: post
// TAG: teams
// REQUEST: SplitTeamMemberRequest
// RESPONSE: 204,NoContent
func (rs *TeamResource) SplitMemberHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	data := &SplitTeamMemberRequest{}
	if err := render.Bind(r, data); err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	if err := rs.checkTutorAuthorization(accessClaims.LoginID, data.UserID, course.ID, courseRole); err != nil {
		_ = render.Render(w, r, ErrUnauthorizedWithDetails(err))
		return
	}

	if _, err := rs.Stores.Team.SplitToSolo(data.UserID, course.ID); err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	render.Status(r, http.StatusNoContent)
}

// CreateCustomTeamHandler creates a custom team grouping arbitrary students.
// URL: /courses/{course_id}/teams
// METHOD: post
// TAG: teams
// REQUEST: CreateCustomTeamRequest
// RESPONSE: 204,NoContent
func (rs *TeamResource) CreateCustomTeamHandler(w http.ResponseWriter, r *http.Request) {
	course := r.Context().Value(symbol.CtxKeyCourse).(*model.Course)
	accessClaims := r.Context().Value(symbol.CtxKeyAccessClaims).(*authenticate.AccessClaims)
	courseRole := r.Context().Value(symbol.CtxKeyCourseRole).(authorize.CourseRole)

	data := &CreateCustomTeamRequest{}
	if err := render.Bind(r, data); err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	for _, uid := range data.UserIDs {
		if err := rs.checkTutorAuthorization(accessClaims.LoginID, uid, course.ID, courseRole); err != nil {
			_ = render.Render(w, r, ErrUnauthorizedWithDetails(err))
			return
		}
	}

	_, syncedPairs, err := rs.Stores.Team.CreateCustomTeam(data.UserIDs, course.ID)
	if err != nil {
		_ = render.Render(w, r, ErrBadRequestWithDetails(err))
		return
	}

	for _, pair := range syncedPairs {
		src := helper.NewSubmissionFileHandle(pair[0])
		dst := helper.NewSubmissionFileHandle(pair[1])
		_ = src.CopyTo(dst)
	}

	render.Status(r, http.StatusNoContent)
}
