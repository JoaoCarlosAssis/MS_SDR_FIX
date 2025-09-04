package router

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/models"
)

// ---- Handlers de Client ----

func createClient(c *gin.Context, d Dependencies) {
	var in struct {
		Name            string      `json:"name"`
		SecretID        string      `json:"secretId"`
		WebhookURL      string      `json:"webhookUrl"`
		Plan            models.Plan `json:"plan"`
		RateLimitPerMin int         `json:"rateLimitPerMin"`
		IsActive        *bool       `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	client := &models.Client{
		SecretID:        in.SecretID,
		Name:            in.Name,
		WebhookURL:      in.WebhookURL,
		Plan:            in.Plan,
		RateLimitPerMin: in.RateLimitPerMin,
		IsActive:        true,
	}
	if in.IsActive != nil {
		client.IsActive = *in.IsActive
	}
	if err := d.ClientSvc.Create(c.Request.Context(), client); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"client": client})
}

func listClients(c *gin.Context, d Dependencies) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	items, err := d.ClientSvc.List(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro ao listar"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func getClient(c *gin.Context, d Dependencies) {
	id := c.Param("id")
	cli, err := d.ClientSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"client": cli})
}

func updateClient(c *gin.Context, d Dependencies) {
	id := c.Param("id")
	var in struct {
		Name            string      `json:"name"`
		WebhookURL      string      `json:"webhookUrl"`
		Plan            models.Plan `json:"plan"`
		RateLimitPerMin int         `json:"rateLimitPerMin"`
		IsActive        *bool       `json:"isActive"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	cli := &models.Client{
		ID:              id,
		Name:            in.Name,
		WebhookURL:      in.WebhookURL,
		Plan:            in.Plan,
		RateLimitPerMin: in.RateLimitPerMin,
		IsActive:        true,
	}
	if in.IsActive != nil {
		cli.IsActive = *in.IsActive
	}
	if err := d.ClientSvc.Update(c.Request.Context(), cli); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"client": cli})
}

func deleteClient(c *gin.Context, d Dependencies) {
	id := c.Param("id")
	if err := d.ClientSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func resolveBySecret(c *gin.Context, d Dependencies) {
	secretID := c.Param("secretId")
	// ENV primeiro
	if url, ok := resolveClientWebhookURLFromEnv(secretID); ok {
		c.JSON(http.StatusOK, models.ResolveResponse{WebhookURL: url, RateLimitPerMin: 60, Plan: models.PlanFREE})
		return
	}
	cli, err := d.ClientSvc.GetBySecretID(c.Request.Context(), secretID)
	if err != nil || cli == nil || !cli.IsActive {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cliente não encontrado"})
		return
	}
	c.JSON(http.StatusOK, models.ResolveResponse{
		WebhookURL:      cli.WebhookURL,
		RateLimitPerMin: cli.RateLimitPerMin,
		Plan:            cli.Plan,
	})
}
