package models

import (
	"errors"
	"time"

	"connectrpc.com/connect"
	wv1 "github.com/Dimss/wafie/api/gen/wafie/v1"
	applogger "github.com/Dimss/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IngressRepository struct {
	db      *gorm.DB
	logger  *zap.Logger
	Ingress Ingress
}

func NewIngressRepository(tx *gorm.DB, logger *zap.Logger) *IngressRepository {
	repo := &IngressRepository{db: tx, logger: logger}
	if tx == nil {
		repo.db = db()
	}
	if logger == nil {
		repo.logger = applogger.NewLogger()
	}
	return repo
}

type Ingress struct {
	ID               uint `gorm:"primaryKey"`
	Name             string
	Namespace        string
	Host             string `gorm:"uniqueIndex:idx_ing_host"`
	Port             int32
	Path             string
	ApplicationID    uint        `gorm:"not null"`
	Application      Application `gorm:"foreignKey:ApplicationID"`
	IngressType      uint32
	DiscoveryStatus  uint32
	DiscoveryMessage string    `gorm:"type:text"`
	UpstreamID       string    `gorm:"size:63;not null;index"`
	Upstream         Upstream  `gorm:"foreignKey:UpstreamID;references:ID"`
	CreatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt        time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

func NewIngressFromProto(ingReq *wv1.Ingress) *Ingress {
	return &Ingress{
		Name:             ingReq.Name,
		Namespace:        ingReq.Namespace,
		Path:             ingReq.Path,
		Host:             ingReq.Host,
		Port:             ingReq.Port,
		IngressType:      uint32(ingReq.IngressType),
		ApplicationID:    uint(ingReq.ApplicationId),
		DiscoveryMessage: ingReq.DiscoveryMessage,
		DiscoveryStatus:  uint32(ingReq.DiscoveryStatus),
	}
}

func (s *IngressRepository) Save(ingress *Ingress) error {
	if res := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "host"}},
		DoUpdates: clause.AssignmentColumns(
			[]string{
				"name",
				"namespace",
				"path",
				"port",
				"ingress_type",
				"discovery_message",
				"discovery_status",
				"created_at",
				"updated_at",
			},
		),
	}).Create(ingress); res.Error != nil {
		return connect.NewError(connect.CodeUnknown, res.Error)
	}
	return nil
}

func (i *Ingress) ToProto() *wv1.Ingress {
	return &wv1.Ingress{
		Name:             i.Name,
		Namespace:        i.Namespace,
		Path:             i.Path,
		Host:             i.Host,
		IngressType:      wv1.IngressType(i.IngressType),
		DiscoveryMessage: i.DiscoveryMessage,
		DiscoveryStatus:  wv1.DiscoveryStatusType(i.DiscoveryStatus),
		ApplicationId:    int32(i.ApplicationID),
		Upstream:         i.Upstream.ToProto(),
	}
}

func (i *Ingress) createApplicationIfNotExists(tx *gorm.DB) error {
	app := &Application{}
	if err := tx.Where("name = ?", i.Host).First(app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			appModelSvc := NewApplicationRepository(tx, nil)
			newAppReq := &wv1.CreateApplicationRequest{Name: i.Host}
			appId, err := appModelSvc.CreateApplication(newAppReq)
			if err != nil {
				return err
			}
			i.ApplicationID = appId.ID
			return nil
		}
		if err != nil {
			return err
		}
	}
	i.ApplicationID = app.ID
	return nil
}

func (i *Ingress) BeforeCreate(tx *gorm.DB) error {
	if err := i.createApplicationIfNotExists(tx); err != nil {
		return err
	}
	return nil
}
