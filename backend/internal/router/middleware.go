package router

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/JoaoCarlosAssis/MS_SDR_FIX/internal/notify"
	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	// Força logs de requisições como nível informativo (stdout)
	logger := log.New(os.Stdout, "", log.LstdFlags)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		logger.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}

// Envia alerta no Slack em caso de panic
func RecoveryWithSlack(slackURL string, botToken string, channelID string, errLogger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				errLogger.Printf("panic: %v", r)
				msg := fmt.Sprintf(":rotating_light: Panic em %s %s: %v", c.Request.Method, c.FullPath(), r)
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if slackURL != "" {
						_ = notify.PostSlack(ctx, slackURL, msg)
						return
					}
					if botToken != "" && channelID != "" {
						_ = notify.PostSlackWithBot(ctx, botToken, channelID, msg)
					}
				}()
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
