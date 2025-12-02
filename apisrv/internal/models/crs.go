package models

import (
	"fmt"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

const (
	DefaultCRSProfileName = "default-crs-profile"
	DefaultCrsVersion     = "default"
	DefaultCrsVersionName = "default"
)

type CrsVersion struct {
	ID           uint         `gorm:"primary_key"`
	Name         string       `gorm:"not null"`
	Status       uint32       `gorm:"default:0"`
	Version      string       `gorm:"not null"`
	CrsRuleSets  []CrsRuleSet `gorm:"foreignKey:CrsVersionID;references:ID"`
	ProtectionID uint         `gorm:"not null"`
	Protection   Protection   `gorm:"foreignKey:ProtectionID;references:ID"`
	CreatedAt    time.Time    `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time    `gorm:"default:CURRENT_TIMESTAMP"`
}

type CrsProfile struct {
	ID             uint `gorm:"primaryKey"`
	Name           string
	CrsFileName    string
	CrsFileContent string
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type CrsRuleSet struct {
	ID             uint       `gorm:"primaryKey"`
	CrsFileName    string     `gorm:"not null"`
	CrsFileContent string     `gorm:"not null"`
	CrsVersionID   uint       `gorm:"not null"`
	CrsVersion     CrsVersion `gorm:"foreignKey:CrsVersionID;references:ID"`
	CreatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time  `gorm:"default:CURRENT_TIMESTAMP"`
}

type CRSRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewCrsRepository(tx *gorm.DB, logger *zap.Logger) *CRSRepository {
	repo := &CRSRepository{
		db:     tx,
		logger: logger,
	}
	if tx == nil {
		repo.db = db()
	}
	if logger == nil {
		repo.logger = applogger.NewLogger()
	}
	return repo
}

func (r *CRSRepository) CreateCrsProfile(p *CrsProfile) error {
	return r.db.Create(p).Error
}

func (r *CRSRepository) CreateCrsRuleSet(ruleSet *CrsRuleSet) error {
	return r.db.Create(ruleSet).Error
}

func (r *CRSRepository) GetProfileByName(profileName string) ([]*CrsProfile, error) {
	var profiles []*CrsProfile
	return profiles,
		r.db.Model(&CrsProfile{}).
			Where("name = ?", profileName).
			Find(&profiles).Error
}

func (r *CRSRepository) CreateCrsVersion(crsVersion *CrsVersion) error {
	return r.db.Create(crsVersion).Error
}

func (r *CRSRepository) CloneCrsProfileToCrsRuleSet(profileName string, crsVersionId uint) error {
	profiles, err := r.GetProfileByName(profileName)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		return fmt.Errorf("profile %s not found", profileName)
	}
	for _, profile := range profiles {
		if err := r.CreateCrsRuleSet(&CrsRuleSet{
			CrsFileName:    profile.CrsFileName,
			CrsFileContent: profile.CrsFileContent,
			CrsVersionID:   crsVersionId,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (v *CrsVersion) ToProto() *wv1.CrsVersion {
	crsVersion := &wv1.CrsVersion{
		Id:           uint32(v.ID),
		Name:         v.Name,
		Status:       wv1.CrsVersionStatus(v.Status),
		ProtectionId: uint32(v.ProtectionID),
	}
	// set crs rules sets
	for _, ruleSet := range v.CrsRuleSets {
		crsVersion.CrsRuleSets = append(crsVersion.CrsRuleSets, ruleSet.ToProto())
	}
	return crsVersion
}

func (s *CrsRuleSet) ToProto() *wv1.CrsRuleSet {
	return &wv1.CrsRuleSet{
		Id:             uint32(s.ID),
		CrsFileName:    s.CrsFileName,
		CrsFileContent: s.CrsFileContent,
		CrsVersionId:   uint32(s.CrsVersionID),
	}
}
