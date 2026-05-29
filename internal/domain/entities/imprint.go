package entities

import "time"

// ImprintCategory classifies a long-term memory fact for filtering and display.
type ImprintCategory string

const (
	ImprintCategoryPreference ImprintCategory = "preference"
	ImprintCategoryPersonal   ImprintCategory = "personal"
	ImprintCategoryBusiness   ImprintCategory = "business"
	ImprintCategoryContext    ImprintCategory = "context"
)

// ImprintSource indicates whether a fact was extracted automatically by the LLM
// or set explicitly by the user or application.
type ImprintSource string

const (
	ImprintSourceExtracted ImprintSource = "extracted"
	ImprintSourceExplicit  ImprintSource = "explicit"
)

// Imprint is a long-term memory fact tied to a user and Spirit.
// Facts persist across sessions and are injected into the system prompt on each interaction.
type Imprint struct {
	ID         string          `bson:"_id" json:"id"`
	UserKey    string          `bson:"user_key" json:"user_key"`
	SpiritName string          `bson:"spirit_name" json:"spirit_name"`
	Fact       string          `bson:"fact" json:"fact"`
	Category   ImprintCategory `bson:"category" json:"category"`
	Source     ImprintSource   `bson:"source" json:"source"`
	CreatedAt  time.Time       `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time       `bson:"updated_at" json:"updated_at"`
}
