package models

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/wafieio/wafie/apisrv/pkg/seclang"
	"io"
	"regexp"
	"strconv"
	"text/template"
	"time"

	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	FullCRSProfileName    = "full"
	EmptyCRSProfileName   = "empty"
	DefaultCRSProfileName = EmptyCRSProfileName
	DefaultCrsVersionTag  = "default"
	CustomRuleIdStart     = 10000
	CustomRuleIdEnd       = 99999
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

func (r *CRSRepository) CreateRule(ruleSetId uint, rules []string, appName string) error {
	crsRuleSet := &CrsRuleSet{}
	r.db.Raw(`select r.* from applications as a
    				join protections as p on p.application_id = a.id
              		join crs_versions as c on c.protection_id = p.id
              		join crs_rule_sets as r on c.id = r.crs_version_id
    				where a.name = ? and r.id = ?`, appName, ruleSetId).
		Scan(&crsRuleSet)
	if crsRuleSet.ID == 0 {
		r.logger.Info("not found", zap.String("appName", appName), zap.Uint("ruleSetId", ruleSetId))
		return fmt.Errorf("not found")
	}
	ruleId, err := nextRuleId(crsRuleSet.CrsFileContent)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		ruleObj, err := seclang.ParseRule(rule)
		if err != nil {
			return err
		}
		ruleObj.AddAction(
			seclang.Action{
				Name:      seclang.ActionID,
				Parameter: strconv.FormatUint(ruleId, 10)},
		)
		modSecRule := ruleObj.ToModSecurityRule()
		r.logger.Info("adding modsecurity rule", zap.String("rule", modSecRule))
		crsRuleSet.CrsFileContent += fmt.Sprintf("\n%s", modSecRule)
	}
	return r.db.Save(crsRuleSet).Error
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
		if err := r.CreateCrsRuleSet(&CrsRuleSet{
			CrsFileName:    rule.CrsFileName,
			CrsFileContent: rule.CrsFileContent,
			CrsVersionID:   crsVersionId,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *CRSRepository) ProtectionDesiredState(crsVersionId uint) (*ProtectionDesiredState, error) {
	p := &Protection{}
	if err := r.db.Model(&Protection{}).
		Preload("CrsVersions", "id = ? AND status = ?", crsVersionId, 2).
		First(p).Error; err != nil {
		return nil, err
	}
	//err := ruleSet.Render(&p.DesiredState)
	return &p.DesiredState, nil
}

func (v *CrsVersion) ToProto() *wv1.CrsVersion {
	repo := NewCrsRepository(nil, nil)
	data, err := repo.ProtectionDesiredState(v.ID)
	if err != nil {
		repo.logger.Error(err.Error())
	}
	id := uint32(v.ID)
	crsVersion := &wv1.CrsVersion{
		Id:           &id,
		Tag:          v.Tag,
		Status:       wv1.CrsVersionStatus(v.Status),
		ProtectionId: uint32(v.ProtectionID),
	}
	// set crs rules sets
	for _, ruleSet := range v.CrsRuleSets {
		if protoRule := ruleSet.ToProto(data); protoRule != nil {
			crsVersion.CrsRuleSets = append(crsVersion.CrsRuleSets, protoRule)
		}
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

func (s *CrsRuleSet) ToProto(data *ProtectionDesiredState) *wv1.CrsRuleSet {
	l := NewCrsRepository(nil, nil).logger
	// render rule file
	renderedCrsFileContent, err := s.Render(data.ModSec)
	if err != nil {
		l.Error(err.Error())
		return nil
	}
	// calculate rule MD5
	h := md5.New()
	if _, err := io.WriteString(h, renderedCrsFileContent); err != nil {
		l.Error(err.Error())
		return nil
	}
	return &wv1.CrsRuleSet{
		Id:             uint32(s.ID),
		CrsFileName:    s.CrsFileName,
		CrsFileContent: renderedCrsFileContent,
		CrsVersionId:   uint32(s.CrsVersionID),
		Md5:            hex.EncodeToString(h.Sum(nil)),
	}
}

func (s *CrsRuleSet) Render(data *ModSec) (renderedCrsRules string, err error) {

	tmpl, err := template.New(s.CrsFileName).
		Delims("{{{", "}}}").
		Parse(s.CrsFileContent)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func nextRuleId(crsFileContent string) (uint64, error) {
	re := regexp.MustCompile(`id:\s*(\d+)`)
	matches := re.FindAllStringSubmatch(crsFileContent, -1)
	maxId := 0
	for _, match := range matches {
		if len(match) == 2 {
			id, err := strconv.Atoi(match[1])
			if err != nil {
				return 0, err
			}
			if id > maxId {
				maxId = id
			}
		}
	}
	maxId++ // increase by one
	return uint64(maxId), nil

}
