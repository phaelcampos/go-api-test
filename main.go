package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Task struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Done        bool   `json:"done"`
}

var (
	tasks  = make(map[int]Task)
	nextID = 1
	mutex  = &sync.RWMutex{}
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	var taskInput struct {
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&taskInput); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}
	if taskInput.Description == "" {
		http.Error(w, `{"error": "Missing 'description' in request body"}`, http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	newTask := Task{
		ID:          nextID,
		Description: taskInput.Description,
		Done:        false,
	}
	tasks[nextID] = newTask
	nextID++

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTask)
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	mutex.RLock()
	defer mutex.RUnlock()

	taskList := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, task)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(taskList)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "taskID")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	mutex.RLock()
	task, exists := tasks[id]
	mutex.RUnlock()

	if !exists {
		http.Error(w, `{"error": "Task not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "taskID")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	var taskInput struct {
		Description *string `json:"description"`
		Done        *bool   `json:"done"`
	}

	if err := json.NewDecoder(r.Body).Decode(&taskInput); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	task, exists := tasks[id]
	if !exists {
		http.Error(w, `{"error": "Task not found"}`, http.StatusNotFound)
		return
	}

	if taskInput.Description != nil {
		task.Description = *taskInput.Description
	}
	if taskInput.Done != nil {
		task.Done = *taskInput.Done
	}

	tasks[id] = task

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "taskID")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	mutex.Lock()
	_, exists := tasks[id]
	if exists {
		delete(tasks, id)
	}
	mutex.Unlock()

	if !exists {
		http.Error(w, `{"error": "Task not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthCheckHandler)

	r.Route("/tasks", func(r chi.Router) {
		r.Post("/", createTaskHandler)
		r.Get("/", getTasksHandler)
		r.Get("/{taskID}", getTaskHandler)
		r.Put("/{taskID}", updateTaskHandler)
		r.Delete("/{taskID}", deleteTaskHandler)
	})

	port := "8080"
	log.Printf("Starting server on port %s\n", port)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), r)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
