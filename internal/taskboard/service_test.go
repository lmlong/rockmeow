package taskboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestService(t *testing.T) {
	// Create temp database
	tmpDir, err := os.MkdirTemp("", "taskboard-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "tasks.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewService(store)

	// Test CreateTask
	task, err := service.CreateTaskFromUserRequest("session-1", "Hello, how are you?")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Title == "" {
		t.Error("Task title should not be empty")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Expected pending status, got %s", task.Status)
	}
	if task.Source != TaskSourceUser {
		t.Errorf("Expected user source, got %s", task.Source)
	}

	// Test GetTask
	gotTask, err := service.GetTask(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotTask.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, gotTask.ID)
	}

	// Test StartTask
	if err := service.StartTask(task.ID); err != nil {
		t.Fatal(err)
	}
	startedTask, _ := service.GetTask(task.ID)
	if startedTask.Status != TaskStatusRunning {
		t.Errorf("Expected running status, got %s", startedTask.Status)
	}
	if startedTask.Column != ColumnInProgress {
		t.Errorf("Expected in_progress column, got %s", startedTask.Column)
	}

	// Test CompleteTask
	if err := service.CompleteTask(task.ID, "Test result"); err != nil {
		t.Fatal(err)
	}
	completedTask, _ := service.GetTask(task.ID)
	if completedTask.Status != TaskStatusCompleted {
		t.Errorf("Expected completed status, got %s", completedTask.Status)
	}
	if completedTask.Column != ColumnDone {
		t.Errorf("Expected done column, got %s", completedTask.Column)
	}
	if completedTask.Result != "Test result" {
		t.Errorf("Expected result 'Test result', got %s", completedTask.Result)
	}

	// Test ListTasks
	tasks, err := service.ListTasks(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// Test GetStats
	stats, err := service.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 1 {
		t.Errorf("Expected total 1, got %d", stats.Total)
	}
	if stats.ByStatus["completed"] != 1 {
		t.Errorf("Expected 1 completed task, got %d", stats.ByStatus["completed"])
	}

	// Test GetBoard
	board, err := service.GetBoard()
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Done) != 1 {
		t.Errorf("Expected 1 task in done, got %d", len(board.Done))
	}

	// Test DeleteTask
	if err := service.DeleteTask(task.ID); err != nil {
		t.Fatal(err)
	}
	_, err = service.GetTask(task.ID)
	if err == nil {
		t.Error("Expected error when getting deleted task")
	}
}

func TestCreateSubagentTask(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskboard-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "tasks.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewService(store)

	task, err := service.CreateSubagentTask("sub-123", "Write code", "Context info", "parent-456")
	if err != nil {
		t.Fatal(err)
	}
	if task.Source != TaskSourceSubagent {
		t.Errorf("Expected subagent source, got %s", task.Source)
	}
	if task.SourceRef != "sub-123" {
		t.Errorf("Expected sourceRef 'sub-123', got %s", task.SourceRef)
	}
	if task.Metadata["parentTaskId"] != "parent-456" {
		t.Errorf("Expected parentTaskId in metadata")
	}
}

func TestCreateCronTask(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskboard-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "tasks.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := NewService(store)

	task, err := service.CreateCronTask("cron-789", "Daily Report", "Generate daily report")
	if err != nil {
		t.Fatal(err)
	}
	if task.Source != TaskSourceCron {
		t.Errorf("Expected cron source, got %s", task.Source)
	}
	if task.SourceRef != "cron-789" {
		t.Errorf("Expected sourceRef 'cron-789', got %s", task.SourceRef)
	}
}

func TestCleanupOldTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "taskboard-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "tasks.db")
	sqliteStore, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqliteStore.Close()

	service := NewService(sqliteStore)

	// Create and complete an old task
	oldTask := &Task{
		Title:  "Old task",
		Status: TaskStatusCompleted,
		Column: ColumnDone,
		Source: TaskSourceManual,
	}
	if err := service.CreateTask(oldTask); err != nil {
		t.Fatal(err)
	}
	// Complete the task to set completed_at
	sqliteStore.UpdateStatus(oldTask.ID, TaskStatusCompleted)
	// Manually update completed_at in database to 30 days ago using raw SQL
	sqliteStore.Exec("UPDATE tasks SET completed_at = ? WHERE id = ?", time.Now().AddDate(0, 0, -30), oldTask.ID)

	// Create a new task
	newTask := &Task{
		Title:  "New task",
		Status: TaskStatusPending,
		Column: ColumnTodo,
		Source: TaskSourceManual,
	}
	if err := service.CreateTask(newTask); err != nil {
		t.Fatal(err)
	}

	// Cleanup tasks older than 7 days
	count, err := service.CleanupOldTasks(7)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Expected 1 task cleaned up, got %d", count)
	}

	// Verify old task is deleted
	_, err = service.GetTask(oldTask.ID)
	if err == nil {
		t.Error("Old task should be deleted")
	}

	// Verify new task still exists
	_, err = service.GetTask(newTask.ID)
	if err != nil {
		t.Error("New task should still exist")
	}
}
