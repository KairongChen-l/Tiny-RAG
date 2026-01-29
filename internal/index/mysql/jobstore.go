package mysql

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/krc/rag/internal/job"
)

// JobModel represents a job in the database.
type JobModel struct {
	ID              string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Type            string    `gorm:"type:varchar(50);not null;index:idx_jobs_type_status" json:"type"`
	Status          string    `gorm:"type:varchar(50);not null;index:idx_jobs_status;index:idx_jobs_type_status" json:"status"`
	Progress        int       `gorm:"default:0" json:"progress"`
	CurrentStage    string    `gorm:"type:varchar(100)" json:"current_stage"`
	StageMessage    string    `gorm:"type:text" json:"stage_message"`
	ProgressHistory string    `gorm:"type:json" json:"-"` // JSON string
	Error           string    `gorm:"type:text" json:"error"`
	Result          string    `gorm:"type:text" json:"result"`
	Payload         []byte    `gorm:"type:blob" json:"-"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time `gorm:"index" json:"updated_at"`
}

// TableName returns the table name.
func (JobModel) TableName() string {
	return "jobs"
}

// JobStore implements job.Store using MySQL + GORM.
type JobStore struct {
	db *gorm.DB
}

// NewJobStore creates a new MySQL job store.
func NewJobStore(db *gorm.DB) *JobStore {
	store := &JobStore{db: db}
	// Auto migrate schema
	db.AutoMigrate(&JobModel{})
	return store
}

// Create creates a new job record.
func (s *JobStore) Create(ctx context.Context, j *job.Job) error {
	historyJSON, _ := json.Marshal(j.ProgressHistory)

	model := JobModel{
		ID:              j.ID,
		Type:            string(j.Type),
		Status:          string(j.Status),
		Progress:        j.Progress,
		CurrentStage:    j.CurrentStage,
		StageMessage:    j.StageMessage,
		ProgressHistory: string(historyJSON),
		Error:           j.Error,
		Result:          j.Result,
		Payload:         j.Payload,
		CreatedAt:       j.CreatedAt,
		UpdatedAt:       j.UpdatedAt,
	}

	return s.db.WithContext(ctx).Create(&model).Error
}

// Update updates a job record.
func (s *JobStore) Update(ctx context.Context, j *job.Job) error {
	j.UpdatedAt = time.Now()
	historyJSON, _ := json.Marshal(j.ProgressHistory)

	model := JobModel{
		ID:              j.ID,
		Type:            string(j.Type),
		Status:          string(j.Status),
		Progress:        j.Progress,
		CurrentStage:    j.CurrentStage,
		StageMessage:    j.StageMessage,
		ProgressHistory: string(historyJSON),
		Error:           j.Error,
		Result:          j.Result,
		Payload:         j.Payload,
		UpdatedAt:       j.UpdatedAt,
	}

	return s.db.WithContext(ctx).Model(&JobModel{}).
		Where("id = ?", j.ID).
		Updates(model).Error
}

// Get retrieves a job by ID.
func (s *JobStore) Get(ctx context.Context, id string) (*job.Job, error) {
	var model JobModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, job.ErrJobNotFound
		}
		return nil, err
	}

	var history []job.ProgressEntry
	if model.ProgressHistory != "" {
		json.Unmarshal([]byte(model.ProgressHistory), &history)
	}

	return &job.Job{
		ID:              model.ID,
		Type:            job.Type(model.Type),
		Status:          job.Status(model.Status),
		Progress:        model.Progress,
		CurrentStage:    model.CurrentStage,
		StageMessage:    model.StageMessage,
		ProgressHistory: history,
		Error:           model.Error,
		Result:          model.Result,
		Payload:         model.Payload,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}, nil
}

// List lists jobs with optional filtering.
func (s *JobStore) List(ctx context.Context, filter job.StoreFilter) ([]*job.Job, error) {
	query := s.db.WithContext(ctx).Model(&JobModel{})

	if filter.Type != "" {
		query = query.Where("type = ?", string(filter.Type))
	}

	if filter.Status != "" {
		query = query.Where("status = ?", string(filter.Status))
	}

	query = query.Order("created_at DESC")

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var models []JobModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	jobs := make([]*job.Job, len(models))
	for i, model := range models {
		var history []job.ProgressEntry
		if model.ProgressHistory != "" {
			json.Unmarshal([]byte(model.ProgressHistory), &history)
		}

		jobs[i] = &job.Job{
			ID:              model.ID,
			Type:            job.Type(model.Type),
			Status:          job.Status(model.Status),
			Progress:        model.Progress,
			CurrentStage:    model.CurrentStage,
			StageMessage:    model.StageMessage,
			ProgressHistory: history,
			Error:           model.Error,
			Result:          model.Result,
			Payload:         model.Payload,
			CreatedAt:       model.CreatedAt,
			UpdatedAt:       model.UpdatedAt,
		}
	}

	return jobs, nil
}


