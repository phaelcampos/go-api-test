package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func resetGlobalState() {
	mutex.Lock()
	defer mutex.Unlock()
	tasks = make(map[int]Task)
	nextID = 1
}

func TestHealthCheckHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	healthCheckHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expectedContentType := "application/json"
	if ctype := rr.Header().Get("Content-Type"); ctype != expectedContentType {
		t.Errorf("handler returned wrong content type: got %v want %v",
			ctype, expectedContentType)
	}

	expectedBody := `{"status":"ok"}`
	if body := strings.TrimSpace(rr.Body.String()); body != expectedBody {
		t.Errorf("handler returned unexpected body: got %v want %v",
			body, expectedBody)
	}
}

func TestCreateTaskHandler(t *testing.T) {
	resetGlobalState()

	jsonData := `{"description": "Minha Tarefa de Teste"}`
	reqBody := bytes.NewBufferString(jsonData)

	req, err := http.NewRequest("POST", "/tasks", reqBody)
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createTaskHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var createdTask Task
	err = json.NewDecoder(rr.Body).Decode(&createdTask)
	if err != nil {
		t.Fatalf("Could not decode response body: %v", err)
	}

	if createdTask.Description != "Minha Tarefa de Teste" {
		t.Errorf("handler returned wrong description: got %v want %v",
			createdTask.Description, "Minha Tarefa de Teste")
	}
	if createdTask.ID != 1 {
		t.Errorf("handler returned wrong ID: got %v want %v",
			createdTask.ID, 1)
	}
	if createdTask.Done != false {
		t.Errorf("handler returned wrong done status: got %v want %v",
			createdTask.Done, false)
	}

	mutex.RLock()
	_, exists := tasks[1]
	mutex.RUnlock()
	if !exists {
		t.Errorf("task was not added to the global tasks map")
	}
}

func TestCreateTaskHandler_BadRequest(t *testing.T) {
	resetGlobalState()

	jsonData := `{}`
	reqBody := bytes.NewBufferString(jsonData)
	req, _ := http.NewRequest("POST", "/tasks", reqBody)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createTaskHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for missing description: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestGetTasksHandler(t *testing.T) {
	resetGlobalState()

	req, _ := http.NewRequest("GET", "/tasks", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getTasksHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for empty list: got %v want %v", status, http.StatusOK)
	}
	if body := strings.TrimSpace(rr.Body.String()); body != "[]" {
		t.Errorf("handler returned wrong body for empty list: got %v want %v", body, "[]")
	}

	resetGlobalState()
	mutex.Lock()
	tasks[1] = Task{ID: 1, Description: "Tarefa 1", Done: false}
	tasks[2] = Task{ID: 2, Description: "Tarefa 2", Done: true}
	nextID = 3
	mutex.Unlock()

	req, _ = http.NewRequest("GET", "/tasks", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for populated list: got %v want %v", status, http.StatusOK)
	}

	var taskList []Task
	err := json.NewDecoder(rr.Body).Decode(&taskList)
	if err != nil {
		t.Fatalf("Could not decode response body for populated list: %v", err)
	}
	if len(taskList) != 2 {
		t.Errorf("handler returned wrong number of tasks: got %d want %d", len(taskList), 2)
	}
}

func newRequestWithChiContext(method, target string, body io.Reader, params map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, err
	}

	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}

	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req, nil
}

func TestGetTaskHandler(t *testing.T) {
	resetGlobalState()
	mutex.Lock()
	tasks[1] = Task{ID: 1, Description: "Buscar esta tarefa", Done: false}
	nextID = 2
	mutex.Unlock()

	req, err := newRequestWithChiContext("GET", "/tasks/1", nil, map[string]string{"taskID": "1"})
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getTaskHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for found task: got %v want %v", status, http.StatusOK)
	}

	var foundTask Task
	if err := json.NewDecoder(rr.Body).Decode(&foundTask); err != nil {
		t.Fatalf("Could not decode response body for found task: %v", err)
	}
	if foundTask.ID != 1 || foundTask.Description != "Buscar esta tarefa" {
		t.Errorf("handler returned wrong task data: got %+v", foundTask)
	}

	req, _ = newRequestWithChiContext("GET", "/tasks/99", nil, map[string]string{"taskID": "99"})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for not found task: got %v want %v", status, http.StatusNotFound)
	}

	req, _ = newRequestWithChiContext("GET", "/tasks/abc", nil, map[string]string{"taskID": "abc"})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid ID: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestUpdateTaskHandler(t *testing.T) {
	resetGlobalState()
	mutex.Lock()
	tasks[1] = Task{ID: 1, Description: "Original", Done: false}
	nextID = 2
	mutex.Unlock()

	updateData := `{"description": "Atualizada", "done": true}`
	reqBody := bytes.NewBufferString(updateData)
	req, err := newRequestWithChiContext("PUT", "/tasks/1", reqBody, map[string]string{"taskID": "1"})
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(updateTaskHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for update success: got %v want %v", status, http.StatusOK)
	}

	var updatedTask Task
	if err := json.NewDecoder(rr.Body).Decode(&updatedTask); err != nil {
		t.Fatalf("Could not decode response body for update success: %v", err)
	}
	if updatedTask.Description != "Atualizada" || !updatedTask.Done || updatedTask.ID != 1 {
		t.Errorf("handler returned wrong updated task data: got %+v", updatedTask)
	}

	mutex.RLock()
	taskInMap := tasks[1]
	mutex.RUnlock()
	if taskInMap.Description != "Atualizada" || !taskInMap.Done {
		t.Errorf("task data in global map was not updated correctly: got %+v", taskInMap)
	}

	updateData = `{"description": "Não importa"}`
	reqBody = bytes.NewBufferString(updateData)
	req, _ = newRequestWithChiContext("PUT", "/tasks/99", reqBody, map[string]string{"taskID": "99"})
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for update not found: got %v want %v", status, http.StatusNotFound)
	}
}

func TestDeleteTaskHandler(t *testing.T) {
	resetGlobalState()
	mutex.Lock()
	tasks[1] = Task{ID: 1, Description: "Para Deletar", Done: false}
	nextID = 2
	mutex.Unlock()

	req, err := newRequestWithChiContext("DELETE", "/tasks/1", nil, map[string]string{"taskID": "1"})
	if err != nil {
		t.Fatalf("Could not create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(deleteTaskHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("handler returned wrong status code for delete success: got %v want %v", status, http.StatusNoContent)
	}

	mutex.RLock()
	_, exists := tasks[1]
	mutex.RUnlock()
	if exists {
		t.Errorf("task was not deleted from the global tasks map")
	}

	req, _ = newRequestWithChiContext("DELETE", "/tasks/99", nil, map[string]string{"taskID": "99"})
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for delete not found: got %v want %v", status, http.StatusNotFound)
	}
}
