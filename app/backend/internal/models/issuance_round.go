package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// IssuanceRound is a named list of graduates of one university (usually one
// graduation decision). Diplomas are attached to a round when they are imported,
// and batch issuing, signing, anchoring and revoking act on one round at a time.
type IssuanceRound struct {
	ID             primitive.ObjectID `bson:"_id" json:"id"`
	UniversityID   primitive.ObjectID `bson:"university_id" json:"-"`
	Name           string             `bson:"name" json:"name"`
	NameKey        string             `bson:"name_key" json:"-"` // lower-cased name, unique per university
	DecisionNumber string             `bson:"decision_number,omitempty" json:"decision_number,omitempty"`
	DecisionDate   *time.Time         `bson:"decision_date,omitempty" json:"decision_date,omitempty"`
	Note           string             `bson:"note,omitempty" json:"note,omitempty"`
	CreatedBy      string             `bson:"created_by,omitempty" json:"-"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
}

type CreateIssuanceRoundRequest struct {
	Name           string `json:"name"`
	DecisionNumber string `json:"decision_number"`
	DecisionDate   string `json:"decision_date"`
	Note           string `json:"note"`
}

// RoundFilterNone selects diplomas that are not attached to any round yet.
const RoundFilterNone = "none"
