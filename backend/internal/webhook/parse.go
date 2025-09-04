package webhook

import (
	"context"
	"encoding/json"
	"log"
	"strings"
)

type eventFields struct {
	Info map[string]any `json:"Info"`
	// Message used for group heuristics
	Message map[string]any `json:"Message"`
}

type jsonDataEnvelope struct {
	Event eventFields `json:"event"`
}

// ExtractEventInfo detecta IsGroup/Chat/Sender com tolerância a variações (versão legada)
func ExtractEventInfo(jsonDataStr string) (isGroup bool, chat string, sender string, reason string) {
	// 1) Tenta struct tipado
	var env jsonDataEnvelope
	if err := json.Unmarshal([]byte(jsonDataStr), &env); err == nil {
		// Info pode ser map; tentar tipos comuns
		if v, ok := env.Event.Info["IsGroup"]; ok {
			switch t := v.(type) {
			case bool:
				isGroup = t
			case string:
				isGroup = strings.EqualFold(strings.TrimSpace(t), "true")
			}
			if isGroup {
				reason = "is_group_true"
			}
		}
		if v, ok := env.Event.Info["Chat"].(string); ok {
			chat = v
		}
		if v, ok := env.Event.Info["Sender"].(string); ok {
			sender = v
		}

		lowerChat := strings.ToLower(chat)
		if strings.Contains(lowerChat, "@g.us") {
			isGroup = true
			if reason == "" {
				reason = "chat_has_g_us"
			}
		}
		if strings.Contains(lowerChat, "@broadcast") || strings.Contains(lowerChat, "status@broadcast") {
			isGroup = true
			if reason == "" {
				reason = "chat_status_broadcast"
			}
		}

		if env.Event.Message != nil {
			if skdm, ok := env.Event.Message["senderKeyDistributionMessage"].(map[string]any); ok {
				if gid, ok := skdm["groupID"].(string); ok {
					lowerGid := strings.ToLower(gid)
					if strings.Contains(lowerGid, "@broadcast") || strings.Contains(lowerGid, "status@broadcast") || strings.Contains(lowerGid, "@g.us") {
						isGroup = true
						if reason == "" {
							reason = "message_group_id_broadcast"
						}
					}
				}
			}
		}
		return isGroup, chat, sender, reason
	}

	// Fallback para dados mal formados
	return false, "", "", "json_parse_failed"
}

// ExtractEventInfoWithConversion extrai informações do evento com conversão automática LID->JID
func ExtractEventInfoWithConversion(ctx context.Context, jsonDataStr string, converter *LIDConverter, apiToken string, logger *log.Logger) (isGroup bool, chat string, sender string, reason string, conversions map[string]string, eventInfo *ConvertedEventInfo) {
	conversions = make(map[string]string)

	if logger != nil {
		logger.Printf("🔍 [WEBHOOK] Iniciando análise do evento (tamanho: %d bytes)", len(jsonDataStr))
	}

	// Se não há conversor configurado, usa função legada
	if converter == nil {
		if logger != nil {
			logger.Printf("⚠️ [WEBHOOK] Conversor LID não configurado, usando parser legado")
		}
		isGroup, chat, sender, reason = ExtractEventInfo(jsonDataStr)
		return isGroup, chat, sender, reason, conversions, nil
	}

	// Debug: Log do token sendo usado
	if logger != nil {
		if apiToken != "" {
			logger.Printf("🔑 [WEBHOOK] Usando token do cliente para conversão (token presente)")
		} else {
			logger.Printf("⚠️ [WEBHOOK] Token do cliente não encontrado - conversões podem falhar")
		}
	}

	// Usa o novo conversor
	convertedEventInfo, err := converter.DetectAndConvertIDs(ctx, jsonDataStr, apiToken)
	if err != nil {
		if logger != nil {
			logger.Printf("❌ [WEBHOOK] Erro na conversão LID->JID: %v", err)
		}
		// Fallback para função legada
		isGroup, chat, sender, reason = ExtractEventInfo(jsonDataStr)

		// Retorna erro para tracking de métricas
		conversions["conversion_error"] = err.Error()
		return isGroup, chat, sender, reason, conversions, nil
	}

	// Extrai dados convertidos
	isGroup = convertedEventInfo.IsGroup
	chat = convertedEventInfo.GetFinalChat()
	sender = convertedEventInfo.GetFinalSender()
	conversions = convertedEventInfo.Conversions
	eventInfo = convertedEventInfo

	// Determina motivo se é grupo
	if isGroup {
		if strings.Contains(strings.ToLower(chat), "@g.us") {
			reason = "chat_has_g_us"
		} else if strings.Contains(strings.ToLower(chat), "@broadcast") {
			reason = "chat_broadcast"
		} else {
			reason = "is_group_true"
		}
	}

	// Log detalhado
	if logger != nil {
		logger.Printf("✅ [WEBHOOK] Análise concluída: IsGroup=%v, Chat=%s, Sender=%s", isGroup, chat, sender)

		if len(conversions) > 0 {
			logger.Printf("🔄 [WEBHOOK] Conversões realizadas:")
			for field, conversion := range conversions {
				logger.Printf("   - %s: %s", field, conversion)
			}
		} else {
			logger.Printf("📝 [WEBHOOK] Nenhuma conversão LID->JID necessária")
		}

		// Log dos dados originais extraídos
		if eventInfo.Chat != "" || eventInfo.Sender != "" || eventInfo.SenderAlt != "" || eventInfo.RecipientAlt != "" {
			logger.Printf("📊 [WEBHOOK] Dados originais extraídos:")
			if eventInfo.Chat != "" {
				logger.Printf("   - Chat: %s", eventInfo.Chat)
			}
			if eventInfo.Sender != "" {
				logger.Printf("   - Sender: %s", eventInfo.Sender)
			}
			if eventInfo.SenderAlt != "" {
				logger.Printf("   - SenderAlt: %s", eventInfo.SenderAlt)
			}
			if eventInfo.RecipientAlt != "" {
				logger.Printf("   - RecipientAlt: %s", eventInfo.RecipientAlt)
			}
		}
	}

	return isGroup, chat, sender, reason, conversions, eventInfo
}
