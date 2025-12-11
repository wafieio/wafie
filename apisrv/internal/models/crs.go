package models

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"time"
)

const (
	DefaultCRSProfileName = "default-crs-profile"
	DefaultCrsVersionTag  = "default"
)

type CrsVersion struct {
	ID           uint         `gorm:"primary_key"`
	Tag          string       `gorm:"not null;uniqueIndex:idx_tag_protection_id_crc_version"`
	Status       uint32       `gorm:"default:0"`
	CrsRuleSets  []CrsRuleSet `gorm:"foreignKey:CrsVersionID;references:ID"`
	ProtectionID uint         `gorm:"not null;uniqueIndex:idx_tag_protection_id_crc_version"`
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
	MD5            string     `gorm:"not null"`
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

func (r *CRSRepository) GetProfileRulesByName(profileName string) (rules []*CrsProfile) {
	r.db.Model(&CrsProfile{}).
		Where("name = ?", profileName).
		Find(&rules)
	return rules
}

func (r *CRSRepository) CreateCrsVersion(crsVersion *CrsVersion) error {
	return r.db.Create(crsVersion).Error
}

func (r *CRSRepository) uniqueCloneProfileOperation(crsVersionId uint) bool {
	// clone can be done only once
	// and only if no existing crs version
	// created yet in crs rule sets
	err := r.db.Where("crs_version_id = ?", crsVersionId).First(&CrsRuleSet{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// allow profile clone only if crs rule with crs version not exist yet
		return true
	}
	if err != nil {
		r.logger.Error(err.Error())
	}
	return false
}

func (r *CRSRepository) CloneCrsProfileToCrsRuleSet(profileName string, crsVersionId uint) error {
	if !r.uniqueCloneProfileOperation(crsVersionId) {
		r.logger.With(zap.Uint("crsVersionId", crsVersionId)).
			With(zap.String("profileName", profileName)).
			Warn("profile already cloned for the provided crs version")
		return nil
	}
	rules := r.GetProfileRulesByName(profileName)
	if len(rules) == 0 {
		return fmt.Errorf("profile %s does not exist", profileName)
	}
	for _, rule := range rules {
		h := md5.New()
		if _, err := io.WriteString(h, rule.CrsFileContent); err != nil {
			return err
		}
		if err := r.CreateCrsRuleSet(&CrsRuleSet{
			CrsFileName:    rule.CrsFileName,
			CrsFileContent: rule.CrsFileContent,
			CrsVersionID:   crsVersionId,
			MD5:            hex.EncodeToString(h.Sum(nil)),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (v *CrsVersion) ToProto() *wv1.CrsVersion {
	id := uint32(v.ID)
	crsVersion := &wv1.CrsVersion{
		Id:           &id,
		Tag:          v.Tag,
		Status:       wv1.CrsVersionStatus(v.Status),
		ProtectionId: uint32(v.ProtectionID),
	}
	// set crs rules sets
	for _, ruleSet := range v.CrsRuleSets {
		crsVersion.CrsRuleSets = append(crsVersion.CrsRuleSets, ruleSet.ToProto())
	}
	return crsVersion
}

func (v *CrsVersion) FromProto(crsVersion *wv1.CrsVersion) {
	if crsVersion == nil {
		return
	}
	if crsVersion.Id != nil {
		v.ID = uint(*crsVersion.Id)
	}
	v.Tag = crsVersion.Tag
	v.Status = uint32(crsVersion.Status)
	v.ProtectionID = uint(crsVersion.ProtectionId)
}

func (s *CrsRuleSet) ToProto() *wv1.CrsRuleSet {
	return &wv1.CrsRuleSet{
		Id:             uint32(s.ID),
		CrsFileName:    s.CrsFileName,
		CrsFileContent: s.CrsFileContent,
		CrsVersionId:   uint32(s.CrsVersionID),
		Md5:            s.MD5,
	}
}
