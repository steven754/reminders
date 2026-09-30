package medkit

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smallgo/server/response"
	"smallgo/server/sysconfig"
)

type aiMedicine struct {
	Name          string `json:"name"`
	GenericName   string `json:"generic_name"`
	Specification string `json:"specification"`
	Quantity      string `json:"quantity"`
	Unit          string `json:"unit"`
	ExpiryDate    string `json:"expiry_date"`
	Notes         string `json:"notes"`
}

func aiDefaults() AIConfig {
	return AIConfig{Enabled: false, BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"}
}

func aiConfig(db *gorm.DB) (AIConfig, error) {
	config := aiDefaults()
	var row AIConfig
	if err := db.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return config, nil
		}
		return config, err
	}
	config = row
	if row.APIKey != "" {
		plain, err := decryptAIKey(db, row.APIKey)
		if err != nil {
			return config, err
		}
		config.APIKey = plain
	}
	return config, nil
}

func aiCryptoKey(db *gorm.DB) ([]byte, error) {
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || strings.TrimSpace(secret) == "" {
		return nil, errors.New("安装密钥不可用")
	}
	sum := sha256.Sum256([]byte("medkit-ai:" + secret))
	return sum[:], nil
}

func encryptAIKey(db *gorm.DB, plain string) (string, error) {
	key, err := aiCryptoKey(db)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	encoded := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(encoded), nil
}

func decryptAIKey(db *gorm.DB, encoded string) (string, error) {
	key, err := aiCryptoKey(db)
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("AI 密钥格式无效")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("AI 密钥无法解密")
	}
	return string(plain), nil
}

func handleGetAIConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		config, err := aiConfig(db)
		if err != nil {
			response.ErrorInternal(c, "读取 AI 配置失败")
			return
		}
		response.Success(c, gin.H{
			"enabled": config.Enabled, "base_url": config.BaseURL, "model": config.Model,
			"configured": config.APIKey != "", "api_key_masked": maskAIKey(config.APIKey),
		})
	}
}

func handleSaveAIConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Enabled     bool   `json:"enabled"`
			BaseURL     string `json:"base_url"`
			Model       string `json:"model"`
			APIKey      string `json:"api_key"`
			ClearAPIKey bool   `json:"clear_api_key"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			response.ErrorBadRequest(c, "AI 配置参数无效")
			return
		}
		in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
		in.Model = strings.TrimSpace(in.Model)
		if in.BaseURL == "" || in.Model == "" {
			response.ErrorBadRequest(c, "请填写 AI 接口地址和模型名称")
			return
		}
		parsed, err := url.Parse(in.BaseURL)
		if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
			response.ErrorBadRequest(c, "AI 接口地址必须是有效的 http(s) 地址")
			return
		}
		config, err := aiConfig(db)
		if err != nil {
			response.ErrorInternal(c, "读取 AI 配置失败")
			return
		}
		secret := config.APIKey
		if in.ClearAPIKey {
			secret = ""
		} else if strings.TrimSpace(in.APIKey) != "" {
			secret = strings.TrimSpace(in.APIKey)
		}
		encrypted := ""
		if secret != "" {
			encrypted, err = encryptAIKey(db, secret)
			if err != nil {
				response.ErrorInternal(c, "保存 AI 密钥失败")
				return
			}
		}
		row := AIConfig{Enabled: in.Enabled, BaseURL: in.BaseURL, Model: in.Model, APIKey: encrypted}
		if err := db.Where("id = ?", config.ID).Assign(map[string]interface{}{"enabled": row.Enabled, "base_url": row.BaseURL, "model": row.Model, "api_key": row.APIKey}).FirstOrCreate(&row).Error; err != nil {
			response.ErrorInternal(c, "保存 AI 配置失败")
			return
		}
		response.Success(c, gin.H{"enabled": row.Enabled, "base_url": row.BaseURL, "model": row.Model, "configured": secret != "", "api_key_masked": maskAIKey(secret)})
	}
}

func handleAIParse(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Text string `json:"text"`
		}
		if err := c.ShouldBindJSON(&in); err != nil || strings.TrimSpace(in.Text) == "" {
			response.ErrorBadRequest(c, "请输入药品文字")
			return
		}
		if len([]rune(in.Text)) > 5000 {
			response.ErrorBadRequest(c, "AI 录入内容不能超过 5000 个字符")
			return
		}
		config, err := aiConfig(db)
		if err != nil {
			response.ErrorInternal(c, "读取 AI 配置失败")
			return
		}
		if !config.Enabled || config.APIKey == "" {
			response.ErrorBadRequest(c, "请先由管理员启用并配置 AI")
			return
		}
		endpoint := strings.TrimRight(config.BaseURL, "/") + "/chat/completions"
		payload := map[string]interface{}{
			"model": config.Model, "temperature": 0,
			"messages": []map[string]string{
				{"role": "system", "content": "你是药品信息整理助手。只做文字结构化，不诊断、不推荐剂量。请只返回 JSON：{\"medicines\":[{\"name\":\"\",\"generic_name\":\"\",\"specification\":\"\",\"quantity\":\"\",\"unit\":\"\",\"expiry_date\":\"YYYY-MM-DD或空\",\"notes\":\"\"}]}。无法确认的字段留空。"},
				{"role": "user", "content": in.Text},
			},
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			response.ErrorBadRequest(c, "AI 请求地址无效")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			response.ErrorBadRequest(c, "AI 请求失败，请检查接口地址和网络")
			return
		}
		defer resp.Body.Close()
		resultBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			response.ErrorBadRequest(c, fmt.Sprintf("AI 接口返回错误（HTTP %d）", resp.StatusCode))
			return
		}
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(resultBody, &result); err != nil || len(result.Choices) == 0 {
			response.ErrorBadRequest(c, "AI 返回内容无法解析")
			return
		}
		content := cleanAIJSON(result.Choices[0].Message.Content)
		var parsed struct {
			Medicines []aiMedicine `json:"medicines"`
		}
		if err := json.Unmarshal([]byte(content), &parsed); err != nil {
			response.Success(c, gin.H{"content": result.Choices[0].Message.Content, "medicines": []aiMedicine{}})
			return
		}
		response.Success(c, gin.H{"content": result.Choices[0].Message.Content, "medicines": parsed.Medicines})
	}
}

func cleanAIJSON(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

func maskAIKey(key string) string {
	if len(key) <= 8 {
		if key == "" {
			return ""
		}
		return "已配置"
	}
	return key[:4] + "……" + key[len(key)-4:]
}
