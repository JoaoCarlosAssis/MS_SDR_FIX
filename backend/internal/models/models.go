package models

import "time"

type Plan string

const (
	PlanFREE  Plan = "FREE"
	PlanPRO   Plan = "PRO"
	PlanSCALE Plan = "SCALE"
)

type Client struct {
	ID              string    `json:"id"`
	SecretID        string    `json:"secretId"`
	Name            string    `json:"name"`
	WebhookURL      string    `json:"webhookUrl"`
	Plan            Plan      `json:"plan"`
	RateLimitPerMin int       `json:"rateLimitPerMin"`
	IsActive        bool      `json:"isActive"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type ResolveResponse struct {
	WebhookURL      string `json:"webhookUrl"`
	RateLimitPerMin int    `json:"rateLimitPerMin"`
	Plan            Plan   `json:"plan"`
}
