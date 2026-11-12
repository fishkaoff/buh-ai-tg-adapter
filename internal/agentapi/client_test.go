// Тесты HTTP-клиента агента.
package agentapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat" || r.Method != http.MethodPost {
			t.Errorf("неожиданный запрос: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("разбор тела запроса: %v", err)
		}
		if req.ChatID != "123" || req.UserID != "456" || req.Message != "привет" {
			t.Errorf("неожиданное тело: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reply":"ответ","operation_id":"op-1","operation_status":"awaiting_confirmation","files":[{"name":"schet_0000-000001.pdf","content_type":"application/pdf","size":7,"kind":"invoice","data_base64":"aGVsbG93b3JsZA=="}]}`))
	}))
	defer server.Close()

	c := New(server.URL, 0)
	resp, err := c.Chat(context.Background(), "123", "456", "привет")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Reply != "ответ" || resp.OperationID == nil || *resp.OperationID != "op-1" {
		t.Fatalf("неожиданный ответ: %+v", resp)
	}
	if len(resp.Files) != 1 || resp.Files[0].Name != "schet_0000-000001.pdf" {
		t.Fatalf("неожиданные файлы: %+v", resp.Files)
	}
	data, err := resp.Files[0].Data()
	if err != nil {
		t.Fatalf("Data(): %v", err)
	}
	if string(data) != "helloworld" {
		t.Fatalf("неожиданное содержимое файла: %q", string(data))
	}
}

func TestConfirmReject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/operations/"+uuid+"/confirm" {
			t.Errorf("неожиданный запрос: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var req operationActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("разбор тела запроса: %v", err)
		}
		if req.ChatID != "123" {
			t.Errorf("неожиданное тело: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"operation_id":"` + uuid + `","status":"confirmed"}`))
	}))
	defer server.Close()

	c := New(server.URL, 0)
	resp, err := c.ConfirmOperation(context.Background(), uuid, "123")
	if err != nil {
		t.Fatalf("ConfirmOperation: %v", err)
	}
	if resp.Status != "confirmed" || resp.OperationID != uuid {
		t.Fatalf("неожиданный ответ: %+v", resp)
	}
}

const uuid = "12345678-1234-1234-1234-123456789012"

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"detail":"Операция уже обработана"}`))
	}))
	defer server.Close()

	c := New(server.URL, 0)
	_, err := c.ConfirmOperation(context.Background(), uuid, "123")
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if !ErrConflict(err) {
		t.Fatalf("ожидалась ошибка 409, получено: %v", err)
	}
}

func TestRejectOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/operations/"+uuid+"/reject" {
			t.Errorf("неожиданный путь: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"operation_id":"` + uuid + `","status":"rejected"}`))
	}))
	defer server.Close()

	c := New(server.URL, 0)
	resp, err := c.RejectOperation(context.Background(), uuid, "123")
	if err != nil {
		t.Fatalf("RejectOperation: %v", err)
	}
	if resp.Status != "rejected" {
		t.Fatalf("неожиданный статус: %+v", resp)
	}
}

func TestInvalidOperationID(t *testing.T) {
	c := New("http://127.0.0.1:1", 0)
	if _, err := c.ConfirmOperation(context.Background(), "../../etc/passwd", "123"); err == nil {
		t.Fatal("ожидалась ошибка для некорректного идентификатора")
	}
}
