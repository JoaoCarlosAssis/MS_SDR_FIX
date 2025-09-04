package service

import (
	"context"
	"errors"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/models"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/repository"
)

type ClientService interface {
	Create(ctx context.Context, c *models.Client) error
	GetByID(ctx context.Context, id string) (*models.Client, error)
	GetBySecretID(ctx context.Context, secretID string) (*models.Client, error)
	List(ctx context.Context, limit, offset int) ([]models.Client, error)
	Update(ctx context.Context, c *models.Client) error
	Delete(ctx context.Context, id string) error
}

type clientService struct {
	repo repository.ClientRepository
}

func NewClientService(repo repository.ClientRepository) ClientService {
	return &clientService{repo: repo}
}

func (s *clientService) Create(ctx context.Context, c *models.Client) error {
	if c.Name == "" || c.WebhookURL == "" {
		return errors.New("name e webhookUrl são obrigatórios")
	}
	if c.RateLimitPerMin <= 0 {
		c.RateLimitPerMin = 60
	}
	if c.Plan == "" {
		c.Plan = models.PlanFREE
	}
	c.IsActive = true
	return s.repo.Create(ctx, c)
}

func (s *clientService) GetByID(ctx context.Context, id string) (*models.Client, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *clientService) GetBySecretID(ctx context.Context, secretID string) (*models.Client, error) {
	return s.repo.GetBySecretID(ctx, secretID)
}

func (s *clientService) List(ctx context.Context, limit, offset int) ([]models.Client, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *clientService) Update(ctx context.Context, c *models.Client) error {
	if c.ID == "" {
		return errors.New("id é obrigatório")
	}
	if c.Name == "" || c.WebhookURL == "" {
		return errors.New("name e webhookUrl são obrigatórios")
	}
	if c.RateLimitPerMin <= 0 {
		c.RateLimitPerMin = 60
	}
	if c.Plan == "" {
		c.Plan = models.PlanFREE
	}
	return s.repo.Update(ctx, c)
}

func (s *clientService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
