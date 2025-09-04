package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/cache"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/config"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/notify"
	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/service"
)

type Dependencies struct {
	Config      config.Config
	Cache       *cache.MemoryCache[string, string]
	HTTPClient  *http.Client
	ClientSvc   service.ClientService
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
}

func Register(r *gin.Engine, d Dependencies) {
	// Healthcheck simples para orquestradores (Railway, etc.)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Raiz informativa
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "MS_SDR backend",
			"status":  "ok",
			"endpoints": []string{
				"GET /healthz",
				"POST /webhook/:secretId",
				"GET /api/clients",
				"POST /api/clients",
				"GET /api/clients/:id",
				"PATCH /api/clients/:id",
				"DELETE /api/clients/:id",
				"GET /api/clients/by-secret/:secretId",
			},
		})
	})

	// Webhook (data-plane)
	r.POST("/webhook/:secretId", func(c *gin.Context) {
		secretID := strings.TrimSpace(c.Param("secretId"))
		if secretID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "secret id ausente"})
			return
		}

		contentType := c.Request.Header.Get("Content-Type")
		ctLower := strings.ToLower(contentType)
		if !(strings.HasPrefix(ctLower, "application/x-www-form-urlencoded") ||
			strings.HasPrefix(ctLower, "multipart/form-data") ||
			strings.HasPrefix(ctLower, "application/json")) {
			d.InfoLogger.Printf("Aviso: Content-Type inesperado: %s", contentType)
		}

		// Helper p/ notificar erros no Slack (assíncrono)
		notifySlack := func(msg string) {
			if url := strings.TrimSpace(d.Config.SlackWebhookURL); url != "" {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := notify.PostSlack(ctx, url, msg); err != nil {
						d.ErrorLogger.Printf("Slack notify (webhook) error: %v", err)
					}
				}()
			} else if bt, ch := strings.TrimSpace(d.Config.SlackBotToken), strings.TrimSpace(d.Config.SlackChannelID); bt != "" && ch != "" {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := notify.PostSlackWithBot(ctx, bt, ch, msg); err != nil {
						d.ErrorLogger.Printf("Slack notify (bot) error: %v", err)
					}
				}()
			}
		}

		// Leia e preserve o corpo original para suportar multipart/json/urlencoded
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			d.ErrorLogger.Printf("Erro ao ler corpo: %v", err)
			notifySlack(fmt.Sprintf(":warning: Erro ao ler corpo | secretId=%s | err=%v", secretID, err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
			return
		}
		// Restaura o Body para futuras leituras (FormValue/forward)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var jsonDataStr string
		switch {
		case strings.HasPrefix(ctLower, "application/json"):
			// Tentar dois formatos comuns:
			// 1) Corpo inteiro já é o JSON do evento
			// 2) Corpo é um envelope { "jsonData": "{...}" }
			bodyTrim := strings.TrimSpace(string(bodyBytes))
			if bodyTrim != "" {
				// Primeiro tenta extrair de envelope (prioridade)
				type kv map[string]any
				var m kv
				if err := json.Unmarshal(bodyBytes, &m); err == nil {
					// Debug: mostra as chaves encontradas
					d.InfoLogger.Printf("🔍 [DEBUG] Chaves no envelope: %v", getMapKeysFromAny(m))

					// Verifica se existe campo jsonData ou body.jsonData
					if v, ok := m["jsonData"].(string); ok && strings.TrimSpace(v) != "" {
						d.InfoLogger.Printf("🔍 [DEBUG] Extraindo de m[jsonData]")
						jsonDataStr = v
					} else if body, ok := m["body"].(map[string]interface{}); ok {
						d.InfoLogger.Printf("🔍 [DEBUG] Tentando extrair de m[body] - chaves: %v", getMapKeysFromAny(body))

						// Caso especial: body tem uma chave que é o JSON inteiro
						for key, value := range body {
							if strings.HasPrefix(key, "{") && strings.HasSuffix(key, "}") {
								d.InfoLogger.Printf("🔍 [DEBUG] Encontrou JSON como chave em body")
								jsonDataStr = key
								break
							}
							if v, ok := value.(string); ok && strings.HasPrefix(v, "{") {
								d.InfoLogger.Printf("🔍 [DEBUG] Encontrou JSON como valor em body")
								jsonDataStr = v
								break
							}
						}

						// Fallback: procura jsonData tradicional
						if jsonDataStr == "" {
							if v, ok := body["jsonData"].(string); ok && strings.TrimSpace(v) != "" {
								d.InfoLogger.Printf("🔍 [DEBUG] Extraindo de m[body][jsonData]")
								jsonDataStr = v
							} else {
								d.InfoLogger.Printf("🔍 [DEBUG] body sem JSON válido, usando corpo inteiro")
								jsonDataStr = bodyTrim
							}
						}
					} else {
						d.InfoLogger.Printf("🔍 [DEBUG] Nem jsonData nem body encontrados, usando corpo inteiro")
						// Se não tem envelope, usa corpo inteiro como evento
						jsonDataStr = bodyTrim
					}
				} else {
					d.InfoLogger.Printf("🔍 [DEBUG] Erro no parse do envelope: %v", err)
					jsonDataStr = bodyTrim
				}
			}

		case strings.HasPrefix(ctLower, "multipart/form-data"):
			// Parse explícito para garantir acesso a campos
			_ = c.Request.ParseMultipartForm(10 << 20) // 10 MiB
			jsonDataStr = c.Request.FormValue("jsonData")

		default: // urlencoded e outros
			_ = c.Request.ParseForm()
			jsonDataStr = c.Request.FormValue("jsonData")
		}
		// Debug: Log do que foi extraído
		if jsonDataStr != "" {
			preview := jsonDataStr
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			d.InfoLogger.Printf("🔍 [DEBUG] jsonData extraído: %s", preview)
		}

		if strings.TrimSpace(jsonDataStr) == "" {
			// Log auxiliar para depuração em ambientes reais
			bodyPreview := string(bodyBytes)
			if len(bodyPreview) > 256 {
				bodyPreview = bodyPreview[:256] + "..."
			}
			d.ErrorLogger.Printf("Campo 'jsonData' ausente ou vazio | CT=%q | body_len=%d | preview=%q", contentType, len(bodyBytes), bodyPreview)
			notifySlack(fmt.Sprintf(":warning: jsonData ausente | secretId=%s | CT=%s | len=%d", secretID, contentType, len(bodyBytes)))
			c.JSON(http.StatusBadRequest, gin.H{"error": "jsonData ausente"})
			return
		}

		// Valida cliente/secret
		targetURL, ok := resolveClientWebhookCached(c, d, secretID)
		if !ok {
			d.InfoLogger.Printf("{\"event\":\"client_not_found\",\"secret_id\":%q}", secretID)
			notifySlack(fmt.Sprintf(":warning: client_not_found | secretId=%s", secretID))
			c.JSON(http.StatusNotFound, gin.H{"error": "Cliente não encontrado"})
			return
		}

		// Filtro imediato por substring no bruto (cobre JSON double-escapado)
		rawLower := strings.ToLower(jsonDataStr)
		rawCompact := strings.Map(func(r rune) rune {
			switch r {
			case ' ', '\n', '\r', '\t':
				return -1
			default:
				return r
			}
		}, rawLower)
		if strings.Contains(rawLower, "@g.us") || strings.Contains(rawLower, "@broadcast") || strings.Contains(rawLower, "status@broadcast") || strings.Contains(rawCompact, "\"isgroup\":true") || strings.Contains(rawCompact, "\\\"isgroup\\\":true") {
			d.InfoLogger.Printf("{\"event\":\"rejected_group\",\"reason\":\"filter_raw_match\"}")
			c.JSON(http.StatusOK, gin.H{"status": "ignored_group_message"})
			return
		}

		// Dados para envio (sempre os dados originais)
		dataToSend := bodyBytes

		d.InfoLogger.Printf("{\"event\":\"webhook_send\",\"secret_id\":%q,\"size\":%d}", secretID, len(dataToSend))

		// Encaminha dados ao destino preservando Content-Type
		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(dataToSend))
		if err != nil {
			d.ErrorLogger.Printf("Erro criar req: %v", err)
			notifySlack(fmt.Sprintf(":warning: Erro criar req para %s | secretId=%s | err=%v", targetURL, secretID, err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		} else {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err := d.HTTPClient.Do(req)
		if err != nil {
			d.ErrorLogger.Printf("Erro ao encaminhar: %v", err)
			notifySlack(fmt.Sprintf(":warning: Forward falhou para %s | secretId=%s | err=%v", targetURL, secretID, err))
			c.JSON(http.StatusBadGateway, gin.H{"error": "destino indisponível"})
			return
		}
		defer resp.Body.Close()

		c.Status(resp.StatusCode)
		c.Writer.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		_, _ = io.Copy(c.Writer, resp.Body)
	})

	// Admin API - Clients
	g := r.Group("/api")
	{
		g.POST("/clients", func(c *gin.Context) { createClient(c, d) })
		g.GET("/clients", func(c *gin.Context) { listClients(c, d) })
		g.GET("/clients/:id", func(c *gin.Context) { getClient(c, d) })
		g.PATCH("/clients/:id", func(c *gin.Context) { updateClient(c, d) })
		g.DELETE("/clients/:id", func(c *gin.Context) { deleteClient(c, d) })

		// Resolver mínimo para data-plane (cache/ENV fallback ocorre no handler de webhook)
		g.GET("/clients/by-secret/:secretId", func(c *gin.Context) { resolveBySecret(c, d) })
	}

	// Opcional: Purge de cache por secretId protegido por token
	r.POST("/admin/cache/purge/:secretId", func(c *gin.Context) {
		token := c.Request.Header.Get("x-admin-key")
		if strings.TrimSpace(d.Config.AdminServiceToken) == "" || token != d.Config.AdminServiceToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		secretID := strings.TrimSpace(c.Param("secretId"))
		if secretID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "secretId requerido"})
			return
		}
		d.Cache.Delete(secretID)
		c.Status(http.StatusNoContent)
	})
}

// resolveClientWebhookCached tenta cache -> ENV -> repo
func resolveClientWebhookCached(c *gin.Context, d Dependencies, secretID string) (string, bool) {
	if v, ok := d.Cache.Get(secretID); ok {
		return v, true
	}
	// ENV compatível
	if url, ok := resolveClientWebhookURLFromEnv(secretID); ok {
		d.Cache.Set(secretID, url)
		return url, true
	}
	// Repo
	client, err := d.ClientSvc.GetBySecretID(c, secretID)
	if err == nil && client != nil && client.IsActive {
		d.Cache.Set(secretID, client.WebhookURL)
		return client.WebhookURL, true
	}
	return "", false
}

// ENV compatível com formato CLIENT_{UUID}
func resolveClientWebhookURLFromEnv(secretID string) (string, bool) {
	underscored := strings.ReplaceAll(secretID, "-", "_")
	lower := "CLIENT_" + strings.ToLower(underscored)
	upper := "CLIENT_" + strings.ToUpper(underscored)
	if v := strings.TrimSpace(os.Getenv(lower)); v != "" {
		return v, true
	}
	if v := strings.TrimSpace(os.Getenv(upper)); v != "" {
		return v, true
	}
	return "", false
}

// getMapKeysFromAny retorna as chaves de um map para debug
func getMapKeysFromAny(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
