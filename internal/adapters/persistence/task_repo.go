package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"drudge/internal/common"
	"drudge/internal/task"
)

const (
	TasksDirName = "tasks"

	// taskFileExtension is the suffix of every task file in a project.
	taskFileExtension = ".md"

	// taskFileIDSeparator splits a task file name into the task id and the
	// task title.
	taskFileIDSeparator = " "
)

// Front matter keys of a task file.
const (
	metaKeyID              = "id"
	metaKeyTitle           = "title"
	metaKeyStatus          = "status"
	metaKeyTicketID        = "ticket_id"
	metaKeyProjectSlug     = "project_slug"
	metaKeyRunnerID        = "runner_id"
	metaKeyRunnerSessionID = "runner_session_id"
	metaKeyStartedAt       = "started_at"
	metaKeyFinishedAt      = "finished_at"
	metaKeyCreatedAt       = "created_at"
	metaKeyUpdatedAt       = "updated_at"
)

type FileTaskRepository struct {
	Project string
}

func NewFileTaskRepository(project string) *FileTaskRepository {
	return &FileTaskRepository{Project: project}
}

func generateTaskID() (task.TaskID, error) {
	id, err := common.GenerateUUID()
	if err != nil {
		return "", fmt.Errorf("could not generate task id: %w", err)
	}

	return task.TaskID(id), nil
}

func (r *FileTaskRepository) taskDir() string {
	home, err := common.HomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(common.ProjectsDir(home), r.Project, TasksDirName)
}

func (r *FileTaskRepository) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	id, err := generateTaskID()
	if err != nil {
		return nil, fmt.Errorf("could not generate task id: %w", err)
	}

	tasksDir := r.taskDir()
	if err := common.EnsureDir(tasksDir); err != nil {
		return nil, fmt.Errorf("could not create tasks directory: %w", err)
	}

	taskFilePath := filepath.Join(tasksDir, taskFileName(id, dto.Title))

	created := &task.Task{
		ID:          id,
		Title:       dto.Title,
		Description: dto.Description,
		Status:      dto.Status,
		TicketID:    dto.TicketID,
		ProjectSlug: r.Project,
		CreatedAt:   dto.CreatedAt,
	}

	if err := common.WriteFileWithFrontMatter(taskFilePath, taskFrontMatter(created), created.Description); err != nil {
		return nil, fmt.Errorf("could not write task file: %w", err)
	}

	return created, nil
}

// taskFrontMatter serializes a task's fields into front matter keys. Optional
// fields are written only when set, so a task file carries no empty entries.
func taskFrontMatter(taskToWrite *task.Task) map[string]string {
	metadata := map[string]string{
		metaKeyID:          string(taskToWrite.ID),
		metaKeyTitle:       taskToWrite.Title,
		metaKeyStatus:      string(taskToWrite.Status),
		metaKeyProjectSlug: taskToWrite.ProjectSlug,
		metaKeyCreatedAt:   taskToWrite.CreatedAt.Format(time.RFC3339),
	}
	if taskToWrite.TicketID != "" {
		metadata[metaKeyTicketID] = taskToWrite.TicketID
	}
	if taskToWrite.RunnerID != 0 {
		metadata[metaKeyRunnerID] = strconv.Itoa(taskToWrite.RunnerID)
	}
	if taskToWrite.RunnerSessionID != "" {
		metadata[metaKeyRunnerSessionID] = taskToWrite.RunnerSessionID
	}
	if !taskToWrite.StartedAt.IsZero() {
		metadata[metaKeyStartedAt] = taskToWrite.StartedAt.Format(time.RFC3339)
	}
	if !taskToWrite.FinishedAt.IsZero() {
		metadata[metaKeyFinishedAt] = taskToWrite.FinishedAt.Format(time.RFC3339)
	}
	if !taskToWrite.UpdatedAt.IsZero() {
		metadata[metaKeyUpdatedAt] = taskToWrite.UpdatedAt.Format(time.RFC3339)
	}
	return metadata
}

func (r *FileTaskRepository) parseTaskFromFile(path string) (*task.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", path, err)
	}

	metadata, content := common.ParseFrontMatter(string(data))

	t := &task.Task{
		Description: content,
	}

	if id, ok := metadata[metaKeyID]; ok {
		t.ID = task.TaskID(id)
	}
	if title, ok := metadata[metaKeyTitle]; ok {
		t.Title = title
	}
	if status, ok := metadata[metaKeyStatus]; ok {
		t.Status = task.TaskStatus(status)
	}
	if ticketID, ok := metadata[metaKeyTicketID]; ok {
		t.TicketID = ticketID
	}
	if projectSlug, ok := metadata[metaKeyProjectSlug]; ok {
		t.ProjectSlug = projectSlug
	}
	if runnerID, ok := metadata[metaKeyRunnerID]; ok {
		parsed, err := strconv.Atoi(runnerID)
		if err != nil {
			return nil, fmt.Errorf("task file %s has %s = %q, it must be a whole number", path, metaKeyRunnerID, runnerID)
		}
		t.RunnerID = parsed
	}
	if runnerSessionID, ok := metadata[metaKeyRunnerSessionID]; ok {
		t.RunnerSessionID = runnerSessionID
	}
	if startedAt, ok := metadata[metaKeyStartedAt]; ok {
		t.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	}
	if finishedAt, ok := metadata[metaKeyFinishedAt]; ok {
		t.FinishedAt, _ = time.Parse(time.RFC3339, finishedAt)
	}
	if createdAt, ok := metadata[metaKeyCreatedAt]; ok {
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	if updatedAt, ok := metadata[metaKeyUpdatedAt]; ok {
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	}

	return t, nil
}

func (r *FileTaskRepository) ListTasks(projectSlug string) ([]*task.Task, error) {
	tasksDir := r.taskDir()

	exists, err := common.Exists(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not check tasks directory: %w", err)
	}
	if !exists {
		return nil, nil
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks directory: %w", err)
	}

	var tasks []*task.Task
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), taskFileExtension) {
			continue
		}

		t, err := r.parseTaskFromFile(filepath.Join(tasksDir, e.Name()))
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// taskFile is one task as its file name describes it. A task file is named
// after the task it holds, so its id and title are readable straight off a
// directory entry, without opening the file.
type taskFile struct {
	id    task.TaskID
	title string
	path  string
}

// taskFileName names the file a task is stored in.
func taskFileName(id task.TaskID, title string) string {
	return string(id) + taskFileIDSeparator + title + taskFileExtension
}

// describeTaskFile reads back the id and title a task file name carries.
func describeTaskFile(tasksDir string, name string) taskFile {
	base := strings.TrimSuffix(name, taskFileExtension)
	id, title, _ := strings.Cut(base, taskFileIDSeparator)
	return taskFile{
		id:    task.TaskID(id),
		title: title,
		path:  filepath.Join(tasksDir, name),
	}
}

// taskFiles reads the project's task files off its directory entries.
func (r *FileTaskRepository) taskFiles() ([]taskFile, error) {
	tasksDir := r.taskDir()

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("could not list tasks directory: %w", err)
	}

	files := make([]taskFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), taskFileExtension) {
			continue
		}
		files = append(files, describeTaskFile(tasksDir, entry.Name()))
	}
	return files, nil
}

// exactTaskFile picks the file of the task carrying exactly this id.
func (r *FileTaskRepository) exactTaskFile(id task.TaskID) (taskFile, error) {
	if id == "" {
		return taskFile{}, task.ErrNoTaskID
	}

	files, err := r.taskFiles()
	if err != nil {
		return taskFile{}, err
	}

	for _, candidate := range files {
		if candidate.id == id {
			return candidate, nil
		}
	}
	return taskFile{}, task.NotFoundError(string(id))
}

// findTaskFile picks the task file named by a full id or by a prefix of one.
// An id matching a file exactly wins over one that is only a prefix, and the
// match ignores case.
func (r *FileTaskRepository) findTaskFile(fullOrPartialID string) (taskFile, error) {
	// Catchall query is a hard erorr.
	if fullOrPartialID == "" {
		return taskFile{}, task.ErrNoTaskID
	}

	files, err := r.taskFiles()
	if err != nil {
		return taskFile{}, err
	}

	wanted := strings.ToLower(fullOrPartialID)
	var matches []taskFile

	for _, candidate := range files {
		candidateID := strings.ToLower(string(candidate.id))
		if candidateID == wanted {
			return candidate, nil
		}
		if strings.HasPrefix(candidateID, wanted) {
			matches = append(matches, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return taskFile{}, task.NotFoundError(fullOrPartialID)
	case 1:
		return matches[0], nil
	default:
		return taskFile{}, task.AmbiguousIDError(fullOrPartialID, taskIDMatches(matches))
	}
}

// taskIDMatches renders matched task files for an ambiguous id error.
func taskIDMatches(matches []taskFile) []task.IDMatch {
	reported := make([]task.IDMatch, 0, len(matches))
	for _, match := range matches {
		reported = append(reported, task.IDMatch{ID: match.id, Title: match.title})
	}
	return reported
}

// readTaskFile parses the task a file holds.
func (r *FileTaskRepository) readTaskFile(found taskFile) (*task.Task, error) {
	parsed, err := r.parseTaskFromFile(found.path)
	if err != nil {
		return nil, err
	}

	// The file name is what a lookup matches on, and the front matter is what
	// drudge works from. A file renamed by hand creates ID inconsistency
	// and is a hard error due to invalid data.
	if parsed.ID != found.id {
		return nil, fmt.Errorf("task file %s holds task %s, rename it back or fix its id", found.path, parsed.ID)
	}
	return parsed, nil
}

// GetTask reads the task carrying exactly this id.
func (r *FileTaskRepository) GetTask(projectSlug string, id task.TaskID) (*task.Task, error) {
	found, err := r.exactTaskFile(id)
	if err != nil {
		return nil, err
	}
	return r.readTaskFile(found)
}

// FindTask reads the task named by a full id or by a prefix of one.
func (r *FileTaskRepository) FindTask(projectSlug string, fullOrPartialID string) (*task.Task, error) {
	found, err := r.findTaskFile(fullOrPartialID)
	if err != nil {
		return nil, err
	}
	return r.readTaskFile(found)
}

// UpdateTask rewrites the file of an existing task and stamps it as updated.
func (r *FileTaskRepository) UpdateTask(projectSlug string, taskToUpdate *task.Task) error {
	found, err := r.exactTaskFile(taskToUpdate.ID)
	if err != nil {
		return err
	}

	taskToUpdate.UpdatedAt = time.Now().UTC()

	if err := common.WriteFileWithFrontMatter(found.path, taskFrontMatter(taskToUpdate), taskToUpdate.Description); err != nil {
		return fmt.Errorf("could not write task file %s: %w", found.path, err)
	}

	return nil
}
