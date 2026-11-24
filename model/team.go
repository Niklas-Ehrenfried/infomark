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

package model

import (
	"time"

	null "github.com/guregu/null/v6"
)

// Team is a database entity representing a group of students working together on
// a course's team-enabled exercise sheets.
type Team struct {
	ID          int64       `db:"id"`
	CreatedAt   time.Time   `db:"created_at,omitempty"`
	UpdatedAt   time.Time   `db:"updated_at,omitempty"`
	CourseID    int64       `db:"course_id"`
	InviteToken null.String `db:"invite_token"`
}

// TeamMember links a student user to a course team.
type TeamMember struct {
	ID          int64      `db:"id"`
	TeamID      int64      `db:"team_id"`
	UserID      int64      `db:"user_id"`
	CourseID    int64      `db:"course_id"`
	JoinedAt    time.Time  `db:"joined_at"`
	ConsentedAt *time.Time `db:"consented_at"`
}

// TeamMemberView is the PII-minimized peer view shown to teammates.
// According to GDPR requirements, emails and matriculation numbers are never exposed to peers.
type TeamMemberView struct {
	UserID    int64       `json:"user_id" db:"user_id"`
	FirstName string      `json:"first_name" db:"first_name"`
	LastName  string      `json:"last_name" db:"last_name"`
	AvatarURL null.String `json:"avatar_url" db:"avatar_url"`
	JoinedAt  time.Time   `json:"joined_at" db:"joined_at"`
}

// TeamWithMembers encapsulates a team and its peer-visible member list.
type TeamWithMembers struct {
	Team
	Members []TeamMemberView `json:"members"`
}

// TeamRosterMember represents member details in the tutor/admin roster management view.
type TeamRosterMember struct {
	UserID        int64       `json:"user_id" db:"user_id"`
	FirstName     string      `json:"first_name" db:"first_name"`
	LastName      string      `json:"last_name" db:"last_name"`
	Email         string      `json:"email" db:"email"`
	StudentNumber string      `json:"student_number" db:"student_number"`
	AvatarURL     null.String `json:"avatar_url" db:"avatar_url"`
	GroupID       null.Int    `json:"group_id" db:"group_id"`
	GroupDesc     null.String `json:"group_description" db:"group_description"`
	TeamID        int64       `json:"team_id" db:"team_id"`
	TeamNumber    int64       `json:"team_number" db:"team_number"`
	TeamSize      int         `json:"team_size" db:"team_size"`
	JoinedAt      time.Time   `json:"joined_at" db:"joined_at"`
}
