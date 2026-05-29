package entities

import "time"

type Operator struct {
	ID           string    `bson:"_id"           json:"id"`
	Name         string    `bson:"name"          json:"name"`
	Email        string    `bson:"email"         json:"email"`
	Role         string    `bson:"role"          json:"role"`
	PasswordHash string    `bson:"password_hash" json:"-"`
	IsActive     bool      `bson:"is_active"     json:"is_active"`
	CreatedAt    time.Time `bson:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at"    json:"updated_at"`
}
