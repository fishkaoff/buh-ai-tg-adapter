// HTTP-клиент сервиса buh-ai-agent.
package agentapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client — клиент HTTP API агента.
type Client struct {
	baseURL string
	http    *http.Client
}

// New создаёт клиента агента.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// ChatResponse — ответ агента на сообщение.
type ChatResponse struct {
	Reply           string         `json:"reply"`
	OperationID     *string        `json:"operation_id"`
	OperationStatus *string        `json:"operation_status"`
	Files           []ResponseFile `json:"files"`
}

// ResponseFile — файл (PDF печатной формы) в ответе агента, base64.
type ResponseFile struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Kind        string `json:"kind"`
	DataBase64  string `json:"data_base64"`
}

// Data декодирует содержимое файла из base64.
func (f ResponseFile) Data() ([]byte, error) {
	return base64.StdEncoding.DecodeString(f.DataBase64)
}

// StatusResponse — результат смены статуса операции.
type StatusResponse struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
}

// APIError — ошибка HTTP API агента.
type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("агент вернул %d: %s", e.StatusCode, e.Detail)
}

// chatRequest — тело POST /chat.
type chatRequest struct {
	ChatID  string `json:"chat_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// operationActionRequest — тело POST /operations/{id}/confirm|reject.
type operationActionRequest struct {
	ChatID string `json:"chat_id"`
}

// Chat отправляет сообщение пользователя агенту и возвращает ответ.
func (c *Client) Chat(ctx context.Context, chatID, userID, message string) (*ChatResponse, error) {
	var resp ChatResponse
	if err := c.post(ctx, "/chat", chatRequest{ChatID: chatID, UserID: userID, Message: message}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConfirmOperation подтверждает операцию (awaiting_confirmation → confirmed).
func (c *Client) ConfirmOperation(ctx context.Context, operationID, chatID string) (*StatusResponse, error) {
	return c.changeStatus(ctx, operationID, chatID, "confirm")
}

// RejectOperation отклоняет операцию (awaiting_confirmation → rejected).
func (c *Client) RejectOperation(ctx context.Context, operationID, chatID string) (*StatusResponse, error) {
	return c.changeStatus(ctx, operationID, chatID, "reject")
}

func (c *Client) changeStatus(ctx context.Context, operationID, chatID, action string) (*StatusResponse, error) {
	if !validOperationID(operationID) {
		return nil, fmt.Errorf("некорректный идентификатор операции")
	}
	var resp StatusResponse
	if err := c.post(ctx, "/operations/"+operationID+"/"+action, operationActionRequest{ChatID: chatID}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Health проверяет живость сервиса агента.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("агент недоступен: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // тело опустошено ниже
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Detail: "не ok"}
	}
	return nil
}

// post выполняет JSON-запрос и декодирует ответ в out.
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("запрос к агенту не выполнен: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // тело опустошено ниже

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("чтение ответа агента: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode, Detail: parseDetail(raw)}
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("разбор ответа агента: %w", err)
	}
	return nil
}

// parseDetail извлекает поле detail из FastAPI-ответа об ошибке.
func parseDetail(raw []byte) string {
	var v struct {
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(raw, &v); err == nil && v.Detail != "" {
		return v.Detail
	}
	return strings.TrimSpace(string(raw))
}

// validOperationID проверяет, что идентификатор операции — это UUID,
// чтобы не допустить path traversal в URL.
func validOperationID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, ch := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if ch != '-' {
				return false
			}
			continue
		}
		if !strings.ContainsRune("0123456789abcdefABCDEF", ch) {
			return false
		}
	}
	return true
}

// ErrNotFound true, если операция не найдена (404).
func ErrNotFound(err error) bool { return status(err, http.StatusNotFound) }

// ErrForbidden true, если операция принадлежит другому чату (403).
func ErrForbidden(err error) bool { return status(err, http.StatusForbidden) }

// ErrConflict true, если операция уже обработана (409).
func ErrConflict(err error) bool { return status(err, http.StatusConflict) }

func status(err error, code int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == code
	}
	return false
}
