// InfoMark - a platform for managing courses with
//            distributing exercise sheets and testing exercise submissions
// Copyright (C) 2019  Infomark Authors
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

	"github.com/go-chi/render"
	"github.com/infomark-org/infomark/model"
)

// ExamResponse is the response payload for course management.
type ExamResponse struct {
	ID              int64     `json:"id" example:"1"`
	Name            string    `json:"name" example:"Info2"`
	Description     string    `json:"description" example:"Some course description here"`
	ExamTime        time.Time `json:"exam_time" example:"auto"`
	CourseID        int64     `json:"course_id" example:"1"`
	MaxPoints       int       `json:"max_points"`
	BonusEnabled    bool      `json:"bonus_enabled"`
	BonusPercentage int       `json:"bonus_percentage"`
	EnrolledCount   int       `json:"enrolled_count"`
}

// Render post-processes a ExamResponse.
func (body *ExamResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// newExamResponse creates a response from an exam model.
func (rs *ExamResource) newExamResponse(p *model.Exam, counts map[int64]int) *ExamResponse {
	count := 0
	if counts != nil {
		count = counts[p.ID]
	} else if rs.Stores != nil && rs.Stores.Exam != nil {
		count, _ = rs.Stores.Exam.GetEnrolledCount(p.ID)
	}
	return &ExamResponse{
		ID:              p.ID,
		Name:            p.Name,
		Description:     p.Description,
		ExamTime:        p.ExamTime,
		CourseID:        p.CourseID,
		MaxPoints:       p.MaxPoints,
		BonusEnabled:    p.BonusEnabled,
		BonusPercentage: p.BonusPercentage,
		EnrolledCount:   count,
	}
}

// newExamListResponse creates a response from a list of exam models.
func (rs *ExamResource) newExamListResponse(exams []model.Exam, counts map[int64]int) []render.Renderer {
	list := []render.Renderer{}
	for k := range exams {
		list = append(list, rs.newExamResponse(&exams[k], counts))
	}
	return list
}

// ExamEnrollmentResponse is the response payload for course management.
type ExamEnrollmentResponse struct {
	Status      int    `json:"status" example:"1"`
	Mark        string `json:"mark" example:"1"`
	UserID      int64  `json:"user_id" example:"42"`
	CourseID    int64  `json:"course_id" example:"1"`
	ExamID      int64  `json:"exam_id" example:"1"`
	ExamPoints  int    `json:"exam_points"`
	BonusPoints int    `json:"bonus_points"`
	RawMark     string `json:"raw_mark"`
}

// Render post-processes a ExamEnrollmentResponse.
func (body *ExamEnrollmentResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// newExamEnrollmentResponse creates a response from a course model.
func newExamEnrollmentResponse(p *model.UserExam) *ExamEnrollmentResponse {

	return &ExamEnrollmentResponse{
		Status:      p.Status,
		Mark:        p.Mark,
		UserID:      p.UserID,
		CourseID:    p.CourseID,
		ExamID:      p.ExamID,
		ExamPoints:  p.ExamPoints,
		BonusPoints: p.BonusPoints,
		RawMark:     p.RawMark,
	}
}

func newExamEnrollmentListResponse(enrollments []model.UserExam) []render.Renderer {
	list := []render.Renderer{}
	for k := range enrollments {
		list = append(list, newExamEnrollmentResponse(&enrollments[k]))
	}

	return list
}
