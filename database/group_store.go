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
	"sort"

	"github.com/infomark-org/infomark/model"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type GroupStore struct {
	db *sqlx.DB
}

func NewGroupStore(db *sqlx.DB) *GroupStore {
	return &GroupStore{
		db: db,
	}
}

func (s *GroupStore) Get(groupID int64) (*model.Group, error) {
	p := model.Group{ID: groupID}
	err := s.db.Get(&p, "SELECT * FROM groups WHERE id = $1 LIMIT 1;", p.ID)
	return &p, err
}

func (s *GroupStore) GetAll() ([]model.Group, error) {
	p := []model.Group{}
	err := s.db.Select(&p, "SELECT * FROM groups;")
	return p, err
}

func (s *GroupStore) Create(p *model.Group) (*model.Group, error) {
	// create Group
	newID, err := Insert(s.db, "groups", p)
	if err != nil {
		return nil, err
	}

	return s.Get(newID)
}

func (s *GroupStore) Update(p *model.Group) error {
	return Update(s.db, "groups", p.ID, p)
}

func (s *GroupStore) Delete(taskID int64) error {
	return Delete(s.db, "groups", taskID)
}

func (s *GroupStore) GroupsOfCourse(courseID int64) ([]model.GroupWithTutor, error) {
	p := []model.GroupWithTutor{}

	err := s.db.Select(&p, `
SELECT
  g.*,
  u.first_name as tutor_first_name,
  u.last_name as tutor_last_name,
  u.avatar_url as tutor_avatar_url,
  u.email as tutor_email,
  u.language as tutor_language
FROM
  groups g
INNER JOIN users u ON g.tutor_id = u.id
WHERE
  course_id = $1
ORDER BY
  g.id ASC`, courseID)
	return p, err
}

func (s *GroupStore) GetMembers(groupID int64) ([]model.User, error) {
	p := []model.User{}

	err := s.db.Select(&p, `
SELECT
  u.*
FROM
  users u
INNER JOIN
  user_group ug ON ug.user_id = u.id
WHERE
  ug.group_id = $1`, groupID)
	return p, err
}

func (s *GroupStore) GetInCourseWithUser(userID int64, courseID int64) ([]model.GroupWithTutor, error) {
	// This is a list as it is used at the sample places where tutors will get a LIST
	// of their groups (can be multiple ones). For the sake of simplicity, this
	// is  a list as well
	p := []model.GroupWithTutor{}

	err := s.db.Select(&p, `
SELECT
  g.*,
  u.first_name as tutor_first_name,
  u.last_name as tutor_last_name,
  u.avatar_url as tutor_avatar_url,
  u.email as tutor_email,
  u.language as tutor_language
FROM
  groups g
INNER JOIN users u ON g.tutor_id = u.id
INNER JOIN user_group ug on g.id = ug.group_id
WHERE
  course_id = $2
AND
  ug.user_id = $1
ORDER BY
  g.id ASC`, userID, courseID)
	return p, err
}

func (s *GroupStore) EnrolledUsers(
	courseID int64,
	groupID int64,
	roleFilter []string,
	filterFirstName string,
	filterLastName string,
	filterEmail string,
	filterSubject string,
	filterLanguage string) ([]model.UserCourse, error) {
	p := []model.UserCourse{}

	// , u.avatar_path
	err := s.db.Select(&p, `
SELECT
  uc.role,
  u.id,
  u.first_name,
  u.last_name,
  u.email,
  u.student_number,
  u.semester,
  u.subject,
  u.language,
  u.avatar_url
FROM
  user_course uc
INNER JOIN users u ON uc.user_id = u.id
INNER JOIN user_group ug ON ug.user_id = u.id
WHERE
  uc.course_id = $1
AND
  ug.group_id = $2
AND
  uc.role = ANY($3)
AND
  LOWER(u.first_name) LIKE $4
AND
  LOWER(u.last_name) LIKE $5
AND
  LOWER(u.email) LIKE $6
AND
  LOWER(u.subject) LIKE $7
AND
  LOWER(u.language) LIKE $8
    `, courseID, groupID, pq.Array(roleFilter),
		filterFirstName, filterLastName, filterEmail,
		filterSubject, filterLanguage,
	)
	return p, err
}

func (s *GroupStore) GetGroupEnrollmentOfUserInCourse(userID int64, courseID int64) (*model.GroupEnrollment, error) {
	p := &model.GroupEnrollment{}
	err := s.db.Get(p, `
SELECT
  ug.*
FROM
  user_group ug
INNER JOIN groups g ON g.id = ug.group_id
WHERE
  ug.user_id = $1
AND
  g.course_id = $2`, userID, courseID)
	return p, err
}

func (s *GroupStore) CreateGroupEnrollmentOfUserInCourse(p *model.GroupEnrollment) (*model.GroupEnrollment, error) {
	newID, err := Insert(s.db, "user_group", p)
	if err != nil {
		return nil, err
	}

	res := &model.GroupEnrollment{}

	err = s.db.Get(res, `SELECT * FROM user_group WHERE id= $1`, newID)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *GroupStore) ChangeGroupEnrollmentOfUserInCourse(p *model.GroupEnrollment) error {
	return Update(s.db, "user_group", p.ID, p)
}

func (s *GroupStore) AssignStudentToGroup(userID, groupID, courseID int64) error {
	existing, err := s.GetGroupEnrollmentOfUserInCourse(userID, courseID)
	if err == nil && existing != nil {
		existing.GroupID = groupID
		return s.ChangeGroupEnrollmentOfUserInCourse(existing)
	}
	_, err = s.CreateGroupEnrollmentOfUserInCourse(&model.GroupEnrollment{
		UserID:  userID,
		GroupID: groupID,
	})
	return err
}

func (s *GroupStore) RemoveStudentFromGroup(userID, courseID int64) error {
	_, err := s.db.Exec(`
DELETE FROM user_group
WHERE user_id = $1
  AND group_id IN (SELECT id FROM groups WHERE course_id = $2);`, userID, courseID)
	return err
}

func (s *GroupStore) ResetGroupAssignments(courseID int64) error {
	_, err := s.db.Exec(`
DELETE FROM user_group ug
USING groups g
WHERE ug.group_id = g.id
  AND g.course_id = $1;`, courseID)
	return err
}

func (s *GroupStore) AutoAssignStudents(courseID int64, requestedSolver string) (*model.AutoAssignResult, error) {
	var studentIDs []int64
	err := s.db.Select(&studentIDs, `SELECT user_id FROM user_course WHERE course_id = $1 AND role = 0 ORDER BY user_id ASC;`, courseID)
	if err != nil {
		return nil, err
	}
	if len(studentIDs) == 0 {
		return &model.AutoAssignResult{SolverUsed: requestedSolver}, nil
	}

	var groupIDs []int64
	err = s.db.Select(&groupIDs, `SELECT id FROM groups WHERE course_id = $1 ORDER BY id ASC;`, courseID)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return &model.AutoAssignResult{SolverUsed: requestedSolver}, nil
	}

	// Required positive preferences & default solver from course settings
	minPref := 3
	var course model.Course
	if err := s.db.Get(&course, `SELECT group_solver, min_group_preferences FROM courses WHERE id = $1;`, courseID); err == nil {
		if course.MinGroupPreferences > 0 {
			minPref = course.MinGroupPreferences
		}
		if requestedSolver == "" {
			requestedSolver = course.GroupSolver
		}
	}
	if minPref > len(groupIDs) {
		minPref = len(groupIDs)
	}
	if requestedSolver != "regret" && requestedSolver != "greedy" {
		requestedSolver = "optimal"
	}

	var bids []model.GroupBid
	_ = s.db.Select(&bids, `
SELECT gb.user_id, gb.group_id, gb.bid
FROM group_bids gb
INNER JOIN groups g ON gb.group_id = g.id
WHERE g.course_id = $1;`, courseID)

	bidMap := make(map[int64]map[int64]int)
	hasVoted := make(map[int64]bool)
	for _, b := range bids {
		if _, ok := bidMap[b.UserID]; !ok {
			bidMap[b.UserID] = make(map[int64]int)
		}
		bidMap[b.UserID][b.GroupID] = b.Bid
		hasVoted[b.UserID] = true
	}

	// Prepare effective bids with Anti-Gaming / Anti-Hostage protection
	effectiveBids := make(map[int64]map[int64]int)
	for _, uid := range studentIDs {
		effectiveBids[uid] = make(map[int64]int)
		posCount := 0
		for _, gid := range groupIDs {
			val := 5 // neutral default
			if uBids, ok := bidMap[uid]; ok {
				if v, has := uBids[gid]; has {
					val = v
				}
			}
			if val > 0 {
				posCount++
			}
			effectiveBids[uid][gid] = val
		}

		// If student bullet-voted or didn't rate at least minPref groups positively:
		// Anti-hostage rule neutralizes all 0s to 5 so strategic zeros cannot hostage the solver!
		if posCount < minPref {
			for _, gid := range groupIDs {
				if effectiveBids[uid][gid] == 0 {
					effectiveBids[uid][gid] = 5
				}
			}
		}
	}

	// Max capacity per group
	capacity := (len(studentIDs) + len(groupIDs) - 1) / len(groupIDs)
	if capacity < 1 {
		capacity = 1
	}

	var assignment map[int64]int64
	switch requestedSolver {
	case "regret":
		assignment = assignRegretPriority(studentIDs, groupIDs, effectiveBids, capacity)
	case "greedy":
		assignment = assignGreedy(studentIDs, groupIDs, effectiveBids, capacity)
	default:
		assignment = assignMinCostMaxFlow(studentIDs, groupIDs, effectiveBids)
	}

	// Atomically replace previous group assignments within a single transaction.
	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
DELETE FROM user_group ug
USING groups g
WHERE ug.group_id = g.id
  AND g.course_id = $1;`, courseID); err != nil {
		return nil, err
	}

	assignedCount := 0
	votedAssignedCount := 0
	totalSatisfaction := 0.0
	topChoiceCount := 0

	anyVoted := len(hasVoted) > 0
	for _, sid := range studentIDs {
		gid := assignment[sid]
		if gid <= 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO user_group (user_id, group_id) VALUES ($1, $2);`, sid, gid); err != nil {
			return nil, err
		}
		assignedCount++

		// Only count actual voters in satisfaction metrics when at least one student voted,
		// so unbidded default=5 profiles do not artificially skew Top Choice %.
		if !anyVoted || hasVoted[sid] {
			votedAssignedCount++
			actualBid := effectiveBids[sid][gid]
			totalSatisfaction += float64(actualBid)

			maxBid := -1
			for _, ogid := range groupIDs {
				if b := effectiveBids[sid][ogid]; b > maxBid {
					maxBid = b
				}
			}
			if actualBid >= maxBid {
				topChoiceCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	avgSatisfaction := 0.0
	topChoicePct := 0.0
	if votedAssignedCount > 0 {
		avgSatisfaction = float64(int(totalSatisfaction/float64(votedAssignedCount)*10.0+0.5)) / 10.0
		topChoicePct = float64(int((float64(topChoiceCount)/float64(votedAssignedCount)*100.0)*10.0+0.5)) / 10.0
	}

	return &model.AutoAssignResult{
		AssignedCount:   assignedCount,
		SolverUsed:      requestedSolver,
		AvgSatisfaction: avgSatisfaction,
		TopChoicePct:    topChoicePct,
	}, nil
}

func (s *GroupStore) SaveBidsForUser(userID int64, bids map[int64]int) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for gid, bid := range bids {
		res, err := tx.Exec(`UPDATE group_bids SET bid = $3 WHERE user_id = $1 AND group_id = $2;`, userID, gid, bid)
		if err != nil {
			return err
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			if _, err := tx.Exec(`INSERT INTO group_bids (user_id, group_id, bid) VALUES ($1, $2, $3);`, userID, gid, bid); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// assignMinCostMaxFlow solves the exact global optimum maximizing the sum of preference bids.
func assignMinCostMaxFlow(studentIDs []int64, groupIDs []int64, bids map[int64]map[int64]int) map[int64]int64 {
	N := len(studentIDs)
	K := len(groupIDs)
	S := 0
	T := N + K + 1
	numNodes := T + 1

	baseCap := N / K
	ceilCap := (N + K - 1) / K

	type Edge struct {
		to   int
		rev  int
		cap  int
		flow int
		cost int
	}

	adj := make([][]Edge, numNodes)
	addEdge := func(from, to, cap, cost int) {
		adj[from] = append(adj[from], Edge{to: to, rev: len(adj[to]), cap: cap, flow: 0, cost: cost})
		adj[to] = append(adj[to], Edge{to: from, rev: len(adj[from]) - 1, cap: 0, flow: 0, cost: -cost})
	}

	// S -> Student: capacity 1, cost 0
	for i := 0; i < N; i++ {
		addEdge(S, 1+i, 1, 0)
	}

	// Student -> Group: capacity 1, cost = (10 - bid) * 10
	for i, sid := range studentIDs {
		for j, gid := range groupIDs {
			b := bids[sid][gid]
			cost := (10 - b) * 10
			addEdge(1+i, N+1+j, 1, cost)
		}
	}

	// Group -> T: base capacity at cost 0; extra capacity up to ceilCap with tiny cost 1 for balance
	for j := 0; j < K; j++ {
		if baseCap > 0 {
			addEdge(N+1+j, T, baseCap, 0)
		}
		if ceilCap > baseCap {
			addEdge(N+1+j, T, ceilCap-baseCap, 1)
		}
	}

	// Successive Shortest Path with SPFA
	totalFlow := 0
	for totalFlow < N {
		dist := make([]int, numNodes)
		for i := range dist {
			dist[i] = 1e9
		}
		parentVertex := make([]int, numNodes)
		parentEdge := make([]int, numNodes)
		inQueue := make([]bool, numNodes)

		dist[S] = 0
		queue := []int{S}
		inQueue[S] = true

		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			inQueue[u] = false

			for eIdx, e := range adj[u] {
				if e.cap-e.flow > 0 && dist[e.to] > dist[u]+e.cost {
					dist[e.to] = dist[u] + e.cost
					parentVertex[e.to] = u
					parentEdge[e.to] = eIdx
					if !inQueue[e.to] {
						queue = append(queue, e.to)
						inQueue[e.to] = true
					}
				}
			}
		}

		if dist[T] >= 1e8 {
			break
		}

		// Augment flow by 1 unit
		curr := T
		for curr != S {
			p := parentVertex[curr]
			e := parentEdge[curr]
			adj[p][e].flow++
			revIdx := adj[p][e].rev
			adj[curr][revIdx].flow--
			curr = p
		}
		totalFlow++
	}

	assignment := make(map[int64]int64)
	groupCounts := make(map[int64]int)
	for _, gid := range groupIDs {
		groupCounts[gid] = 0
	}

	// Extract assignments from flow network
	for i, sid := range studentIDs {
		studentNode := 1 + i
		assigned := false
		for _, e := range adj[studentNode] {
			if e.to >= N+1 && e.to <= N+K && e.flow == 1 {
				gid := groupIDs[e.to-(N+1)]
				assignment[sid] = gid
				groupCounts[gid]++
				assigned = true
				break
			}
		}
		if !assigned {
			// Fallback: pick least loaded group
			bestGID := groupIDs[0]
			minC := 999999
			for _, gid := range groupIDs {
				if groupCounts[gid] < minC {
					minC = groupCounts[gid]
					bestGID = gid
				}
			}
			assignment[sid] = bestGID
			groupCounts[bestGID]++
		}
	}

	return assignment
}

// assignRegretPriority uses an opportunity cost / regret-based heuristic to assign
// students with high gap between first and second choice before slots fill up.
func assignRegretPriority(studentIDs []int64, groupIDs []int64, bids map[int64]map[int64]int, capacity int) map[int64]int64 {
	groupCounts := make(map[int64]int)
	for _, gid := range groupIDs {
		groupCounts[gid] = 0
	}

	assigned := make(map[int64]bool)
	assignment := make(map[int64]int64)

	for len(assignment) < len(studentIDs) {
		type candidate struct {
			sid      int64
			regret   int
			topBid   int
			topGroup int64
		}

		bestCandidate := candidate{regret: -999999, topBid: -999999}

		for _, sid := range studentIDs {
			if assigned[sid] {
				continue
			}

			type gBid struct {
				gid int64
				bid int
			}
			var avail []gBid
			for _, gid := range groupIDs {
				if groupCounts[gid] < capacity {
					avail = append(avail, gBid{gid: gid, bid: bids[sid][gid]})
				}
			}

			if len(avail) == 0 {
				for _, gid := range groupIDs {
					avail = append(avail, gBid{gid: gid, bid: bids[sid][gid]})
				}
			}

			sort.Slice(avail, func(i, j int) bool {
				if avail[i].bid != avail[j].bid {
					return avail[i].bid > avail[j].bid
				}
				if groupCounts[avail[i].gid] != groupCounts[avail[j].gid] {
					return groupCounts[avail[i].gid] < groupCounts[avail[j].gid]
				}
				return avail[i].gid < avail[j].gid
			})

			topBid := avail[0].bid
			topGroup := avail[0].gid
			regret := 0
			if len(avail) == 1 {
				regret = topBid + 100 // Must assign immediately
			} else {
				regret = topBid - avail[1].bid
			}

			if regret > bestCandidate.regret ||
				(regret == bestCandidate.regret && topBid > bestCandidate.topBid) ||
				(regret == bestCandidate.regret && topBid == bestCandidate.topBid && sid < bestCandidate.sid) {
				bestCandidate = candidate{
					sid:      sid,
					regret:   regret,
					topBid:   topBid,
					topGroup: topGroup,
				}
			}
		}

		assignment[bestCandidate.sid] = bestCandidate.topGroup
		groupCounts[bestCandidate.topGroup]++
		assigned[bestCandidate.sid] = true
	}

	return assignment
}

// assignGreedy assigns students sequentially based on highest preferences.
func assignGreedy(studentIDs []int64, groupIDs []int64, bids map[int64]map[int64]int, capacity int) map[int64]int64 {
	groupCounts := make(map[int64]int)
	for _, gid := range groupIDs {
		groupCounts[gid] = 0
	}

	type sProfile struct {
		sid    int64
		maxBid int
	}
	profiles := make([]sProfile, len(studentIDs))
	for i, sid := range studentIDs {
		mBid := -1
		for _, gid := range groupIDs {
			if b := bids[sid][gid]; b > mBid {
				mBid = b
			}
		}
		profiles[i] = sProfile{sid: sid, maxBid: mBid}
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].maxBid != profiles[j].maxBid {
			return profiles[i].maxBid > profiles[j].maxBid
		}
		return profiles[i].sid < profiles[j].sid
	})

	assignment := make(map[int64]int64)
	for _, p := range profiles {
		sid := p.sid
		bestGID := int64(0)
		bestBid := -1
		bestCount := 999999

		for _, gid := range groupIDs {
			bidVal := bids[sid][gid]
			if groupCounts[gid] < capacity && (bidVal > bestBid || (bidVal == bestBid && groupCounts[gid] < bestCount)) {
				bestBid = bidVal
				bestGID = gid
				bestCount = groupCounts[gid]
			}
		}

		if bestGID == 0 {
			minC := 999999
			for _, gid := range groupIDs {
				if groupCounts[gid] < minC {
					minC = groupCounts[gid]
					bestGID = gid
				}
			}
		}

		assignment[sid] = bestGID
		groupCounts[bestGID]++
	}

	return assignment
}

func (s *GroupStore) GetOfTutor(tutorID int64, courseID int64) ([]model.GroupWithTutor, error) {
	p := []model.GroupWithTutor{}

	err := s.db.Select(&p, `
SELECT
  g.*,
  u.first_name as tutor_first_name,
  u.last_name as tutor_last_name,
  u.avatar_url as tutor_avatar_url,
  u.email as tutor_email,
  u.language as tutor_language
FROM
  groups g
INNER JOIN users u ON g.tutor_id = u.id
WHERE
  course_id = $2
AND
  g.tutor_id = $1
ORDER BY
  g.id ASC`, tutorID, courseID)
	return p, err
}

func (s *GroupStore) IdentifyCourseOfGroup(groupID int64) (*model.Course, error) {

	course := &model.Course{}
	err := s.db.Get(course,
		`
SELECT
  c.*
FROM
  groups g
INNER JOIN courses c ON c.id = g.course_ID
WHERE g.id = $1`,
		groupID)
	if err != nil {
		return nil, err
	}

	return course, err
}

func (s *GroupStore) GetBidOfUserForGroup(userID int64, groupID int64) (bid int, err error) {
	err = s.db.Get(&bid, `
SELECT
  bid
FROM
  group_bids
WHERE
  user_id = $1 and group_id = $2
LIMIT 1`, userID, groupID)
	return bid, err
}

func (s *GroupStore) InsertBidOfUserForGroup(userID int64, groupID int64, bid int) (int, error) {

	// insert
	_, err := Insert(s.db, "group_bids", &model.GroupBid{UserID: userID, GroupID: groupID, Bid: bid})
	if err != nil {
		return 0, err
	}

	return s.GetBidOfUserForGroup(userID, groupID)
}

func (s *GroupStore) UpdateBidOfUserForGroup(userID int64, groupID int64, bid int) (int, error) {

	// update
	_, err := s.db.Exec(`
UPDATE
  group_bids
SET bid = $3
WHERE
  user_id = $1
AND
  group_id = $2`, userID, groupID, bid)
	if err != nil {
		return 0, err
	}

	return s.GetBidOfUserForGroup(userID, groupID)
}

func (s *GroupStore) GetBidsForCourseForUser(courseID int64, userID int64) ([]model.GroupBid, error) {

	p := []model.GroupBid{}

	err := s.db.Select(&p, `
SELECT
  gb.*
FROM
  group_bids gb
INNER JOIN groups g ON gb.group_id = g.id
WHERE
  gb.user_id = $2
AND
  g.course_id = $1`, courseID, userID)
	return p, err

}

func (s *GroupStore) GetBidsForCourse(courseID int64) ([]model.GroupBid, error) {

	p := []model.GroupBid{}

	err := s.db.Select(&p, `
SELECT
  gb.*
FROM
  group_bids gb
INNER JOIN groups g ON gb.group_id = g.id
AND
  g.course_id = $1`, courseID)
	return p, err

}
