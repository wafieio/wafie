package models

import (
	"time"

	"github.com/google/uuid"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type StateRepository struct {
	db          *gorm.DB
	logger      *zap.Logger
	DataVersion StateVersion
}

type StateVersion struct {
	TypeId    uint32 `gorm:"primaryKey"`
	VersionId string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewStateRepository(tx *gorm.DB, logger *zap.Logger) *StateRepository {
	modelSvc := &StateRepository{db: tx, logger: logger}
	if tx == nil {
		modelSvc.db = db()
	}
	if logger == nil {
		modelSvc.logger = applogger.NewLogger()
	}
	return modelSvc
}

func (s *StateRepository) GetVersionByTypeId(typeId uint32) (*StateVersion, error) {
	dv := &StateVersion{}
	if err := s.db.First(dv, typeId).Error; err != nil {
		return nil, err
	}
	return dv, nil
}

func (s *StateRepository) UpdateProtectionVersion() error {
	return s.db.Save(
		&StateVersion{
			TypeId:    uint32(v1.StateTypeId_STATE_TYPE_ID_PROTECTION),
			VersionId: uuid.New().String(),
		},
	).Error
}

func (d *StateVersion) ToProto() *v1.GetStateVersionResponse {
	return &v1.GetStateVersionResponse{
		TypeId:         v1.StateTypeId(d.TypeId),
		StateVersionId: d.VersionId,
	}
}
