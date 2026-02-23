package taskboard

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store 任务存储接口
type Store interface {
	// 任务 CRUD
	Create(task *Task) error
	Get(id string) (*Task, error)
	Update(task *Task) error
	Delete(id string) error
	List(filter *TaskFilter) ([]*Task, error)

	// 批量操作
	UpdateStatus(id string, status TaskStatus) error
	MoveToColumn(id string, column Column) error
	SetAssignee(id string, assignee string, assigneeType AssigneeType) error
	SetError(id string, errMsg string) error
	SetResult(id string, result string) error

	// 统计
	GetStats() (*Stats, error)
	GetBoard() (*Board, error)

	// 事件订阅
	Subscribe() <-chan TaskEvent
	Close() error
}

// SQLiteStore SQLite 存储
type SQLiteStore struct {
	db       *sql.DB
	dbPath   string
	mu       sync.RWMutex
	eventChs []chan TaskEvent
	eventMu  sync.RWMutex
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// 连接数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(1) // SQLite 单连接
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{
		db:     db,
		dbPath: dbPath,
	}

	// 初始化表
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return store, nil
}

// initSchema 初始化数据库结构
func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		priority TEXT NOT NULL DEFAULT 'medium',
		column TEXT NOT NULL DEFAULT 'todo',
		assignee TEXT,
		assignee_type TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		due_date DATETIME,
		started_at DATETIME,
		completed_at DATETIME,
		session_id TEXT,
		source TEXT NOT NULL DEFAULT 'manual',
		source_ref TEXT,
		result TEXT,
		error TEXT,
		metadata TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_tasks_source ON tasks(source);
	CREATE INDEX IF NOT EXISTS idx_tasks_column ON tasks(column);
	CREATE INDEX IF NOT EXISTS idx_tasks_session ON tasks(session_id);
	CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Create 创建任务
func (s *SQLiteStore) Create(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成 ID
	if task.ID == "" {
		task.ID = uuid.New().String()[:8]
	}

	// 设置时间
	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	// 设置默认值
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Priority == "" {
		task.Priority = PriorityMedium
	}
	if task.Column == "" {
		task.Column = ColumnTodo
	}
	if task.Source == "" {
		task.Source = TaskSourceManual
	}

	// 序列化 metadata
	var metadataJSON []byte
	if task.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(task.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		INSERT INTO tasks (
			id, title, description, status, priority, column,
			assignee, assignee_type, created_at, updated_at,
			due_date, started_at, completed_at,
			session_id, source, source_ref, result, error, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		task.ID, task.Title, task.Description, task.Status, task.Priority, task.Column,
		task.Assignee, task.AssigneeType, task.CreatedAt, task.UpdatedAt,
		task.DueDate, task.StartedAt, task.CompletedAt,
		task.SessionID, task.Source, task.SourceRef, task.Result, task.Error, metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeCreate,
		TaskID:    task.ID,
		Task:      task,
		Timestamp: now,
	})

	return nil
}

// Get 获取任务
func (s *SQLiteStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, title, description, status, priority, column,
			assignee, assignee_type, created_at, updated_at,
			due_date, started_at, completed_at,
			session_id, source, source_ref, result, error, metadata
		FROM tasks WHERE id = ?
	`

	task := &Task{}
	var metadataJSON []byte

	err := s.db.QueryRow(query, id).Scan(
		&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.Column,
		&task.Assignee, &task.AssigneeType, &task.CreatedAt, &task.UpdatedAt,
		&task.DueDate, &task.StartedAt, &task.CompletedAt,
		&task.SessionID, &task.Source, &task.SourceRef, &task.Result, &task.Error, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	// 解析 metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
			// 忽略错误，保留 nil
			task.Metadata = nil
		}
	}

	return task, nil
}

// Update 更新任务
func (s *SQLiteStore) Update(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.UpdatedAt = time.Now()

	// 序列化 metadata
	var metadataJSON []byte
	if task.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(task.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	query := `
		UPDATE tasks SET
			title = ?, description = ?, status = ?, priority = ?, column = ?,
			assignee = ?, assignee_type = ?, updated_at = ?,
			due_date = ?, started_at = ?, completed_at = ?,
			session_id = ?, source = ?, source_ref = ?, result = ?, error = ?, metadata = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query,
		task.Title, task.Description, task.Status, task.Priority, task.Column,
		task.Assignee, task.AssigneeType, task.UpdatedAt,
		task.DueDate, task.StartedAt, task.CompletedAt,
		task.SessionID, task.Source, task.SourceRef, task.Result, task.Error, metadataJSON,
		task.ID,
	)

	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeUpdate,
		TaskID:    task.ID,
		Task:      task,
		Timestamp: task.UpdatedAt,
	})

	return nil
}

// Delete 删除任务
func (s *SQLiteStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeDelete,
		TaskID:    id,
		Timestamp: time.Now(),
	})

	return nil
}

// List 列出任务
func (s *SQLiteStore) List(filter *TaskFilter) ([]*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, title, description, status, priority, column,
			assignee, assignee_type, created_at, updated_at,
			due_date, started_at, completed_at,
			session_id, source, source_ref, result, error, metadata
		FROM tasks
	`

	args := []interface{}{}
	conditions := []string{}

	if filter != nil {
		if filter.Status != nil {
			conditions = append(conditions, "status = ?")
			args = append(args, *filter.Status)
		}
		if filter.Source != nil {
			conditions = append(conditions, "source = ?")
			args = append(args, *filter.Source)
		}
		if filter.Column != nil {
			conditions = append(conditions, "column = ?")
			args = append(args, *filter.Column)
		}
		if filter.Priority != nil {
			conditions = append(conditions, "priority = ?")
			args = append(args, *filter.Priority)
		}
		if filter.Assignee != "" {
			conditions = append(conditions, "assignee = ?")
			args = append(args, filter.Assignee)
		}
	}

	if len(conditions) > 0 {
		query += " WHERE "
		for i, cond := range conditions {
			if i > 0 {
				query += " AND "
			}
			query += cond
		}
	}

	query += " ORDER BY created_at DESC"

	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		task := &Task{}
		var metadataJSON []byte

		err := rows.Scan(
			&task.ID, &task.Title, &task.Description, &task.Status, &task.Priority, &task.Column,
			&task.Assignee, &task.AssigneeType, &task.CreatedAt, &task.UpdatedAt,
			&task.DueDate, &task.StartedAt, &task.CompletedAt,
			&task.SessionID, &task.Source, &task.SourceRef, &task.Result, &task.Error, &metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		// 解析 metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
				task.Metadata = nil
			}
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// UpdateStatus 更新状态
func (s *SQLiteStore) UpdateStatus(id string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var startedAt, completedAt *time.Time

	// 如果变为 running，设置 started_at
	if status == TaskStatusRunning {
		startedAt = &now
	}

	// 如果完成或失败，设置 completed_at
	if status == TaskStatusCompleted || status == TaskStatusFailed {
		completedAt = &now
	}

	query := "UPDATE tasks SET status = ?, updated_at = ?"
	args := []interface{}{status, now}

	if startedAt != nil {
		query += ", started_at = ?"
		args = append(args, startedAt)
	}
	if completedAt != nil {
		query += ", completed_at = ?"
		args = append(args, completedAt)
	}

	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeUpdate,
		TaskID:    id,
		Timestamp: now,
	})

	return nil
}

// MoveToColumn 移动到列
func (s *SQLiteStore) MoveToColumn(id string, column Column) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := "UPDATE tasks SET column = ?, updated_at = ? WHERE id = ?"
	result, err := s.db.Exec(query, column, now, id)
	if err != nil {
		return fmt.Errorf("move to column: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeMove,
		TaskID:    id,
		Timestamp: now,
	})

	return nil
}

// SetAssignee 设置分配对象
func (s *SQLiteStore) SetAssignee(id string, assignee string, assigneeType AssigneeType) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := "UPDATE tasks SET assignee = ?, assignee_type = ?, updated_at = ? WHERE id = ?"
	result, err := s.db.Exec(query, assignee, assigneeType, now, id)
	if err != nil {
		return fmt.Errorf("set assignee: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeUpdate,
		TaskID:    id,
		Timestamp: now,
	})

	return nil
}

// SetError 设置错误信息
func (s *SQLiteStore) SetError(id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := "UPDATE tasks SET error = ?, updated_at = ? WHERE id = ?"
	result, err := s.db.Exec(query, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("set error: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task not found: %s", id)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeUpdate,
		TaskID:    id,
		Timestamp: now,
	})

	return nil
}

// SetResult 设置结果
func (s *SQLiteStore) SetResult(id string, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	query := "UPDATE tasks SET result = ?, updated_at = ? WHERE id = ?"
	_, err := s.db.Exec(query, result, now, id)
	if err != nil {
		return fmt.Errorf("set result: %w", err)
	}

	// 发送事件
	s.emitEvent(TaskEvent{
		Type:      EventTypeUpdate,
		TaskID:    id,
		Timestamp: now,
	})

	return nil
}

// GetStats 获取统计信息
func (s *SQLiteStore) GetStats() (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &Stats{
		ByStatus:   make(map[string]int),
		BySource:   make(map[string]int),
		ByColumn:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	// 总数
	err := s.db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&stats.Total)
	if err != nil {
		return nil, fmt.Errorf("count total: %w", err)
	}

	// 按状态
	rows, err := s.db.Query("SELECT status, COUNT(*) FROM tasks GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			stats.ByStatus[status] = count
		}
	}
	rows.Close()

	// 按来源
	rows, err = s.db.Query("SELECT source, COUNT(*) FROM tasks GROUP BY source")
	if err != nil {
		return nil, fmt.Errorf("count by source: %w", err)
	}
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err == nil {
			stats.BySource[source] = count
		}
	}
	rows.Close()

	// 按列
	rows, err = s.db.Query("SELECT column, COUNT(*) FROM tasks GROUP BY column")
	if err != nil {
		return nil, fmt.Errorf("count by column: %w", err)
	}
	for rows.Next() {
		var column string
		var count int
		if err := rows.Scan(&column, &count); err == nil {
			stats.ByColumn[column] = count
		}
	}
	rows.Close()

	// 按优先级
	rows, err = s.db.Query("SELECT priority, COUNT(*) FROM tasks GROUP BY priority")
	if err != nil {
		return nil, fmt.Errorf("count by priority: %w", err)
	}
	for rows.Next() {
		var priority string
		var count int
		if err := rows.Scan(&priority, &count); err == nil {
			stats.ByPriority[priority] = count
		}
	}
	rows.Close()

	return stats, nil
}

// GetBoard 获取看板视图
func (s *SQLiteStore) GetBoard() (*Board, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	board := &Board{
		Backlog:    []*Task{},
		Todo:       []*Task{},
		InProgress: []*Task{},
		Done:       []*Task{},
	}

	tasks, err := s.List(nil)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		switch task.Column {
		case ColumnBacklog:
			board.Backlog = append(board.Backlog, task)
		case ColumnTodo:
			board.Todo = append(board.Todo, task)
		case ColumnInProgress:
			board.InProgress = append(board.InProgress, task)
		case ColumnDone:
			board.Done = append(board.Done, task)
		}
	}

	return board, nil
}

// Subscribe 订阅事件
func (s *SQLiteStore) Subscribe() <-chan TaskEvent {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()

	ch := make(chan TaskEvent, 100)
	s.eventChs = append(s.eventChs, ch)
	return ch
}

// emitEvent 发送事件
func (s *SQLiteStore) emitEvent(event TaskEvent) {
	s.eventMu.RLock()
	defer s.eventMu.RUnlock()

	for _, ch := range s.eventChs {
		select {
		case ch <- event:
		default:
			// 通道满，丢弃
		}
	}
}

// Close 关闭存储
func (s *SQLiteStore) Close() error {
	s.eventMu.Lock()
	for _, ch := range s.eventChs {
		close(ch)
	}
	s.eventChs = nil
	s.eventMu.Unlock()

	return s.db.Close()
}

// Exec 执行原始 SQL（用于测试）
func (s *SQLiteStore) Exec(query string, args ...interface{}) error {
	_, err := s.db.Exec(query, args...)
	return err
}
