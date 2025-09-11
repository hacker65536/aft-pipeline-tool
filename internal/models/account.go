package models

import "time"

// Account represents an AWS account
type Account struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

// AccountCache represents cached account data
type AccountCache struct {
	Accounts []Account `json:"accounts"`
	CachedAt time.Time `json:"cached_at"`
	TTL      int       `json:"ttl"`
}
