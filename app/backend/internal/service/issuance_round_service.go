package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrRoundRequired    = errors.New("Vui lòng chọn đợt cấp")
	ErrRoundNotFound    = errors.New("Không tìm thấy đợt cấp của trường")
	ErrRoundNameInvalid = errors.New("Tên đợt cấp cần từ 3 đến 150 ký tự")
	ErrRoundExists      = errors.New("Tên đợt cấp đã tồn tại")
	ErrFacultyRequired  = errors.New("Vui lòng chọn chuyên ngành")
	ErrRevokeReason     = errors.New("Lý do thu hồi cần từ 5 đến 500 ký tự")
	ErrNothingToRevoke  = errors.New("Không có văn bằng đã cấp nào của đợt và chuyên ngành này để thu hồi")
)

const (
	defaultRoundSuggestions = 5
	maxRoundSuggestions     = 20
)

// CreateRound registers a named issuance round for the university.
func (s *eDiplomaService) CreateRound(ctx context.Context, universityID primitive.ObjectID, req models.CreateIssuanceRoundRequest) (*models.IssuanceRound, error) {
	name := strings.Join(strings.Fields(req.Name), " ")
	if n := utf8.RuneCountInString(name); n < 3 || n > 150 {
		return nil, ErrRoundNameInvalid
	}
	round := &models.IssuanceRound{
		ID:             primitive.NewObjectID(),
		UniversityID:   universityID,
		Name:           name,
		NameKey:        strings.ToLower(name),
		DecisionNumber: strings.TrimSpace(req.DecisionNumber),
		Note:           strings.TrimSpace(req.Note),
		CreatedAt:      time.Now(),
	}
	if value := strings.TrimSpace(req.DecisionDate); value != "" {
		date, err := parseDecisionDate(value)
		if err != nil {
			return nil, errors.New("Ngày quyết định không hợp lệ (dd/mm/yyyy hoặc yyyy-mm-dd)")
		}
		round.DecisionDate = &date
	}
	if claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims); ok {
		round.CreatedBy = claims.AccountID
	}
	if err := s.rounds.Create(ctx, round); err != nil {
		if errors.Is(err, repository.ErrIssuanceRoundExists) {
			return nil, ErrRoundExists
		}
		return nil, err
	}
	return round, nil
}

func parseDecisionDate(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", "02/01/2006"} {
		if date, err := time.Parse(layout, value); err == nil {
			return date, nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

// SearchRounds suggests the newest rounds whose name contains query. With a
// faculty, only rounds that contain diplomas of that faculty are returned.
func (s *eDiplomaService) SearchRounds(ctx context.Context, universityID primitive.ObjectID, query, facultyID string, limit int) ([]models.IssuanceRound, error) {
	if limit <= 0 {
		limit = defaultRoundSuggestions
	}
	if limit > maxRoundSuggestions {
		limit = maxRoundSuggestions
	}
	var onlyIDs []primitive.ObjectID
	if facultyID = strings.TrimSpace(facultyID); facultyID != "" && facultyID != AllFaculties {
		faculty, err := primitive.ObjectIDFromHex(facultyID)
		if err != nil {
			return nil, errors.New("Chuyên ngành không hợp lệ")
		}
		ids, err := s.repo.DistinctRoundIDs(ctx, bson.M{"university_id": universityID, "faculty_id": faculty})
		if err != nil {
			return nil, err
		}
		onlyIDs = ids
	}
	return s.rounds.Search(ctx, universityID, query, onlyIDs, int64(limit))
}

// RequireRound returns the round of the university named by roundID.
func (s *eDiplomaService) RequireRound(ctx context.Context, universityID primitive.ObjectID, roundID string) (*models.IssuanceRound, error) {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" || roundID == models.RoundFilterNone {
		return nil, ErrRoundRequired
	}
	id, err := primitive.ObjectIDFromHex(roundID)
	if err != nil {
		return nil, ErrRoundNotFound
	}
	round, err := s.rounds.FindByID(ctx, universityID, id)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrRoundNotFound
	}
	return round, err
}

// AllFaculties selects every faculty of a round; an empty faculty means the same.
const AllFaculties = "all"

// RoundScope builds the filter "this round, and this faculty or all of them" that
// signing, anchoring and revoking share. The round is required; the user decides
// whether to act on one faculty or on the whole round.
func (s *eDiplomaService) RoundScope(ctx context.Context, universityID primitive.ObjectID, facultyID, roundID string) (bson.M, error) {
	round, err := s.RequireRound(ctx, universityID, roundID)
	if err != nil {
		return nil, err
	}
	scope := bson.M{"university_id": universityID, "round_id": round.ID}
	if facultyID = strings.TrimSpace(facultyID); facultyID != "" && facultyID != AllFaculties {
		faculty, err := primitive.ObjectIDFromHex(facultyID)
		if err != nil {
			return nil, errors.New("Chuyên ngành không hợp lệ")
		}
		scope["faculty_id"] = faculty
	}
	return scope, nil
}

// AssignRound attaches the faculty's diplomas that have no round yet to a round.
func (s *eDiplomaService) AssignRound(ctx context.Context, universityID primitive.ObjectID, roundID, facultyID, course, certificateType string) (int64, error) {
	round, err := s.RequireRound(ctx, universityID, roundID)
	if err != nil {
		return 0, err
	}
	// Same matching as the diploma search, so the records the user sees are the ones assigned.
	filter := bson.M{}
	if facultyID = strings.TrimSpace(facultyID); facultyID != "" && facultyID != AllFaculties {
		faculty, err := primitive.ObjectIDFromHex(facultyID)
		if err != nil {
			return 0, errors.New("Chuyên ngành không hợp lệ")
		}
		filter["faculty_id"] = faculty
	}
	if value := strings.TrimSpace(course); value != "" {
		filter["course"] = bson.M{"$regex": regexp.QuoteMeta(value), "$options": "i"}
	}
	if value := strings.TrimSpace(certificateType); value != "" {
		filter["certificate_type"] = bson.M{"$regex": regexp.QuoteMeta(value), "$options": "i"}
	}
	return s.repo.AssignRound(ctx, filter, round.ID)
}

// RevokeRound revokes every issued diploma of one faculty in one round.
func (s *eDiplomaService) RevokeRound(ctx context.Context, universityID primitive.ObjectID, facultyID, roundID, reason string) (int64, error) {
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < 5 || n > 500 {
		return 0, ErrRevokeReason
	}
	scope, err := s.RoundScope(ctx, universityID, facultyID, roundID)
	if err != nil {
		return 0, err
	}
	// Load the diplomas first so exactly these are revoked and their students notified.
	scope["issued"] = true
	scope["revoked"] = bson.M{"$ne": true}
	diplomas, err := s.repo.FindByDynamicFilter(ctx, scope)
	if err != nil {
		return 0, err
	}
	if len(diplomas) == 0 {
		return 0, ErrNothingToRevoke
	}
	ids := make([]primitive.ObjectID, 0, len(diplomas))
	for _, d := range diplomas {
		ids = append(ids, d.ID)
	}
	revokedAt := time.Now()
	count, err := s.repo.RevokeMany(ctx, bson.M{"_id": bson.M{"$in": ids}}, reason, revokedAt)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, ErrNothingToRevoke
	}
	s.notifier.NotifyRevoked(ctx, diplomas, reason, revokedAt)
	return count, nil
}

// attachRounds fills the round name of each response from one round lookup.
func (s *eDiplomaService) attachRounds(ctx context.Context, diplomas []*models.EDiploma, dtos []*models.EDiplomaResponse) {
	if s.rounds == nil || len(diplomas) == 0 {
		return
	}
	ids := []primitive.ObjectID{}
	for _, d := range diplomas {
		if !d.RoundID.IsZero() {
			ids = append(ids, d.RoundID)
		}
	}
	rounds, err := s.rounds.FindByIDs(ctx, diplomas[0].UniversityID, ids)
	if err != nil {
		return
	}
	for i, d := range diplomas {
		dtos[i].RevocationOnChain = d.RevocationOnChain
		if round, ok := rounds[d.RoundID]; ok {
			dtos[i].RoundID = round.ID.Hex()
			dtos[i].RoundName = round.Name
		}
	}
}
