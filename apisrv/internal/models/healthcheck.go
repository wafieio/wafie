package models

import (
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type HealthCheckModelSvc struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewHealthCheckSvc(tx *gorm.DB, logger *zap.Logger) *HealthCheckModelSvc {
	if tx == nil {
		tx = db()
	}
	if logger == nil {
		logger = applogger.NewLogger()
	}
	return &HealthCheckModelSvc{
		db:     tx,
		logger: logger,
	}
}

func (s *HealthCheckModelSvc) Ping() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Ping(); err != nil {
		return err
	}
	return nil
}
