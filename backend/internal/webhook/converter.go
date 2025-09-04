package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LIDConvertRequest representa a requisição para conversão LID->JID
type LIDConvertRequest struct {
	LID string `json:"lid"`
}

// LIDConvertResponse representa a resposta da API de conversão
type LIDConvertResponse struct {
	Status bool `json:"status"`
	Data   struct {
		Success  bool   `json:"success"`
		JID      string `json:"jid"`
		JIDClear string `json:"jidClear"`
	} `json:"data"`
}

// LIDConverter gerencia conversões LID->JID
type LIDConverter struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewLIDConverter cria uma nova instância do conversor (token será passado dinamicamente)
func NewLIDConverter(baseURL string) *LIDConverter {
	return &LIDConverter{
		BaseURL:    strings.TrimSuffix(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// IsLID verifica se um ID está no formato LID
func IsLID(id string) bool {
	return strings.HasSuffix(strings.ToLower(id), "@lid")
}

// IsJID verifica se um ID está no formato JID
func IsJID(id string) bool {
	lower := strings.ToLower(id)
	return strings.Contains(lower, "@s.whatsapp.net") || strings.Contains(lower, "@g.us")
}

// ConvertLIDToJID converte um LID para JID usando a API Avisa com token dinâmico
func (c *LIDConverter) ConvertLIDToJID(ctx context.Context, lid string, apiToken string) (string, error) {
	if !IsLID(lid) {
		return lid, nil // Não é LID, retorna como está
	}

	if c.BaseURL == "" {
		return "", fmt.Errorf("URL da API Avisa não configurada")
	}

	if apiToken == "" {
		return "", fmt.Errorf("token da API não fornecido para conversão LID->JID")
	}

	// Prepara a requisição
	req := LIDConvertRequest{LID: lid}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("erro ao serializar requisição: %w", err)
	}

	// Cria requisição HTTP
	url := fmt.Sprintf("%s/user/parselid", c.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("erro ao criar requisição HTTP: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Adiciona token de autenticação se fornecido
	if apiToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiToken)
	}

	// Executa requisição
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("erro ao executar requisição: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Lê corpo da resposta para melhor debug
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return "", fmt.Errorf("API retornou status %d, corpo: %s", resp.StatusCode, bodyStr)
	}

	// Parse da resposta
	var response LIDConvertResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("erro ao decodificar resposta: %w", err)
	}

	if !response.Status || !response.Data.Success {
		return "", fmt.Errorf("conversão falhou na API")
	}

	return response.Data.JID, nil
}

// DetectAndConvertIDs extrai e converte todos os IDs relevantes de um evento com token dinâmico
func (c *LIDConverter) DetectAndConvertIDs(ctx context.Context, jsonDataStr string, apiToken string) (*ConvertedEventInfo, error) {
	// Parse inicial do evento
	var env jsonDataEnvelope
	if err := json.Unmarshal([]byte(jsonDataStr), &env); err != nil {
		return nil, fmt.Errorf("erro ao fazer parse do evento: %w", err)
	}

	result := &ConvertedEventInfo{
		OriginalData: jsonDataStr,
		Conversions:  make(map[string]string),
	}

	// Extrai IDs relevantes
	if info := env.Event.Info; info != nil {
		// Chat
		if chat, ok := info["Chat"].(string); ok && chat != "" {
			result.Chat = chat
			if IsLID(chat) {
				if jid, err := c.ConvertLIDToJID(ctx, chat, apiToken); err == nil {
					result.ChatJID = jid
					result.Conversions["Chat"] = fmt.Sprintf("%s -> %s", chat, jid)
				}
			} else {
				result.ChatJID = chat
			}
		}

		// Sender
		if sender, ok := info["Sender"].(string); ok && sender != "" {
			result.Sender = sender
			if IsLID(sender) {
				if jid, err := c.ConvertLIDToJID(ctx, sender, apiToken); err == nil {
					result.SenderJID = jid
					result.Conversions["Sender"] = fmt.Sprintf("%s -> %s", sender, jid)
				}
			} else {
				result.SenderJID = sender
			}
		}

		// SenderAlt (formato LID)
		if senderAlt, ok := info["SenderAlt"].(string); ok && senderAlt != "" {
			result.SenderAlt = senderAlt
			if IsLID(senderAlt) {
				if jid, err := c.ConvertLIDToJID(ctx, senderAlt, apiToken); err == nil {
					result.SenderAltJID = jid
					result.Conversions["SenderAlt"] = fmt.Sprintf("%s -> %s", senderAlt, jid)
					// Se Sender original não existe ou é vazio, usa SenderAlt convertido
					if result.SenderJID == "" {
						result.SenderJID = jid
					}
				}
			}
		}

		// RecipientAlt (formato LID)
		if recipientAlt, ok := info["RecipientAlt"].(string); ok && recipientAlt != "" {
			result.RecipientAlt = recipientAlt
			if IsLID(recipientAlt) {
				if jid, err := c.ConvertLIDToJID(ctx, recipientAlt, apiToken); err == nil {
					result.RecipientAltJID = jid
					result.Conversions["RecipientAlt"] = fmt.Sprintf("%s -> %s", recipientAlt, jid)
					// Se Chat original não existe ou é vazio, usa RecipientAlt convertido
					if result.ChatJID == "" {
						result.ChatJID = jid
					}
				}
			}
		}

		// IsGroup
		if isGroup, ok := info["IsGroup"]; ok {
			switch v := isGroup.(type) {
			case bool:
				result.IsGroup = v
			case string:
				result.IsGroup = strings.EqualFold(strings.TrimSpace(v), "true")
			}
		}
	}

	// Verifica se Chat contém indicadores de grupo
	if result.ChatJID != "" {
		lowerChat := strings.ToLower(result.ChatJID)
		if strings.Contains(lowerChat, "@g.us") || strings.Contains(lowerChat, "@broadcast") {
			result.IsGroup = true
		}
	}

	return result, nil
}

// ConvertedEventInfo contém informações de um evento com IDs convertidos
type ConvertedEventInfo struct {
	OriginalData    string            `json:"originalData"`
	Chat            string            `json:"chat"`
	ChatJID         string            `json:"chatJid"`
	Sender          string            `json:"sender"`
	SenderJID       string            `json:"senderJid"`
	SenderAlt       string            `json:"senderAlt"`
	SenderAltJID    string            `json:"senderAltJid"`
	RecipientAlt    string            `json:"recipientAlt"`
	RecipientAltJID string            `json:"recipientAltJid"`
	IsGroup         bool              `json:"isGroup"`
	Conversions     map[string]string `json:"conversions"`
}

// GetFinalChat retorna o Chat final para uso (prioriza conversões)
func (c *ConvertedEventInfo) GetFinalChat() string {
	if c.ChatJID != "" {
		return c.ChatJID
	}
	return c.Chat
}

// GetFinalSender retorna o Sender final para uso (prioriza conversões)
func (c *ConvertedEventInfo) GetFinalSender() string {
	if c.SenderJID != "" {
		return c.SenderJID
	}
	return c.Sender
}

// HasConversions verifica se houve alguma conversão
func (c *ConvertedEventInfo) HasConversions() bool {
	return len(c.Conversions) > 0
}

// ApplyConversionsToJSON aplica as conversões LID->JID no JSON original
func (c *ConvertedEventInfo) ApplyConversionsToJSON() (string, error) {
	if !c.HasConversions() {
		// Se não há conversões, retorna o JSON original
		return c.OriginalData, nil
	}

	// Parse do JSON original
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(c.OriginalData), &jsonData); err != nil {
		return "", fmt.Errorf("erro ao fazer parse do JSON original: %w", err)
	}

	// Aplica conversões nos campos relevantes
	if event, ok := jsonData["event"].(map[string]interface{}); ok {
		if info, ok := event["Info"].(map[string]interface{}); ok {
			// Aplica conversão no Chat se houver
			if c.ChatJID != "" && c.Chat != "" && c.ChatJID != c.Chat {
				info["Chat"] = c.ChatJID
			}

			// Aplica conversão no Sender se houver
			if c.SenderJID != "" && c.Sender != "" && c.SenderJID != c.Sender {
				info["Sender"] = c.SenderJID
			}

			// Aplica conversão no SenderAlt se houver
			if c.SenderAltJID != "" && c.SenderAlt != "" && c.SenderAltJID != c.SenderAlt {
				info["SenderAlt"] = c.SenderAltJID
			}

			// Aplica conversão no RecipientAlt se houver
			if c.RecipientAltJID != "" && c.RecipientAlt != "" && c.RecipientAltJID != c.RecipientAlt {
				info["RecipientAlt"] = c.RecipientAltJID
			}
		}
	}

	// Serializa o JSON modificado
	modifiedJSON, err := json.Marshal(jsonData)
	if err != nil {
		return "", fmt.Errorf("erro ao serializar JSON modificado: %w", err)
	}

	return string(modifiedJSON), nil
}

// getMapKeys retorna as chaves de um map para debug
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
