package repository

import (
	"context"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClientRepository interface {
	Create(ctx context.Context, c *models.Client) error
	GetByID(ctx context.Context, id string) (*models.Client, error)
	GetBySecretID(ctx context.Context, secretID string) (*models.Client, error)
	List(ctx context.Context, limit, offset int) ([]models.Client, error)
	Update(ctx context.Context, c *models.Client) error
	Delete(ctx context.Context, id string) error
}

type clientRepository struct{ db *pgxpool.Pool }

func NewClientRepository(db *pgxpool.Pool) ClientRepository { return &clientRepository{db: db} }

func (r *clientRepository) Create(ctx context.Context, c *models.Client) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.SecretID == "" {
		c.SecretID = uuid.NewString()
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO clients(id, secret_id, name, webhook_url, plan, rate_limit_per_min, is_active)
        VALUES($1,$2,$3,$4,$5,$6,$7)`, c.ID, c.SecretID, c.Name, c.WebhookURL, c.Plan, c.RateLimitPerMin, c.IsActive); err != nil {
		return err
	}
	return nil
}

func (r *clientRepository) GetByID(ctx context.Context, id string) (*models.Client, error) {
	row := r.db.QueryRow(ctx, `SELECT id, secret_id, name, webhook_url, plan, rate_limit_per_min, is_active, created_at, updated_at FROM clients WHERE id=$1`, id)
	var c models.Client
	if err := row.Scan(&c.ID, &c.SecretID, &c.Name, &c.WebhookURL, &c.Plan, &c.RateLimitPerMin, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *clientRepository) GetBySecretID(ctx context.Context, secretID string) (*models.Client, error) {
	row := r.db.QueryRow(ctx, `SELECT id, secret_id, name, webhook_url, plan, rate_limit_per_min, is_active, created_at, updated_at FROM clients WHERE secret_id=$1`, secretID)
	var c models.Client
	if err := row.Scan(&c.ID, &c.SecretID, &c.Name, &c.WebhookURL, &c.Plan, &c.RateLimitPerMin, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *clientRepository) List(ctx context.Context, limit, offset int) ([]models.Client, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `SELECT id, secret_id, name, webhook_url, plan, rate_limit_per_min, is_active, created_at, updated_at FROM clients ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Client
	for rows.Next() {
		var c models.Client
		if err := rows.Scan(&c.ID, &c.SecretID, &c.Name, &c.WebhookURL, &c.Plan, &c.RateLimitPerMin, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *clientRepository) Update(ctx context.Context, c *models.Client) error {
	if _, err := r.db.Exec(ctx, `UPDATE clients SET name=$1, webhook_url=$2, plan=$3, rate_limit_per_min=$4, is_active=$5, updated_at=NOW() WHERE id=$6`, c.Name, c.WebhookURL, c.Plan, c.RateLimitPerMin, c.IsActive, c.ID); err != nil {
		return err
	}
	return nil
}

func (r *clientRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM clients WHERE id=$1`, id); err != nil {
		return err
	}
	return nil
}
