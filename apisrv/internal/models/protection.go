package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProtectionRepository struct {
	db         *gorm.DB
	logger     *zap.Logger
	Protection Protection
}

type ModSec struct {
	Mode          uint32 `json:"protectionMode"`
	ParanoiaLevel uint32 `json:"paranoiaLevel"`
}

type ProtectionDesiredState struct {
	ModSec            *ModSec `json:"modSec"`
	XffNumTrustedHops uint32  `json:"xffNumTrustedHops"`
}

type Protection struct {
	ID            uint                   `gorm:"primaryKey"`
	Mode          uint32                 `gorm:"default:0"`
	ApplicationID uint                   `gorm:"not null;uniqueIndex:idx_protection_app_id"`
	Application   Application            `gorm:"foreignKey:ApplicationID;references:ID"`
	CrsVersions   []CrsVersion           `gorm:"foreignKey:ProtectionID;references:ID"`
	DesiredState  ProtectionDesiredState `gorm:"type:jsonb"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewProtectionRepository(tx *gorm.DB, logger *zap.Logger) *ProtectionRepository {
	modelSvc := &ProtectionRepository{db: tx, logger: logger}

	if tx == nil {
		modelSvc.db = db()
	}
	if logger == nil {
		modelSvc.logger = applogger.NewLogger()
	}

	return modelSvc
}

func (s *ProtectionDesiredState) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return fmt.Errorf("unsupported type for ProtectionDesiredState")
	}
}

func (s *ProtectionDesiredState) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *ProtectionDesiredState) FromProto(v1desiredState *wv1.ProtectionDesiredState) {
	s.ModSec = &ModSec{
		Mode:          uint32(v1desiredState.ModeSec.ProtectionMode),
		ParanoiaLevel: uint32(v1desiredState.ModeSec.ParanoiaLevel),
	}
	s.XffNumTrustedHops = v1desiredState.XffNumTrustedHops
}

func (s *ProtectionDesiredState) ToProto() *wv1.ProtectionDesiredState {
	return nil
}

func (p *Protection) FromProto(protectionv1 *wv1.Protection) error {
	if protectionv1 == nil {
		return fmt.Errorf("protection is required")
	}
	p.Mode = uint32(protectionv1.ProtectionMode)
	p.ApplicationID = uint(protectionv1.ApplicationId)
	p.DesiredState.FromProto(protectionv1.DesiredState)
	return nil
}

func (p *Protection) ToProto() *wv1.Protection {

	protection := &wv1.Protection{
		Id:             uint32(p.ID),
		ApplicationId:  uint32(p.ApplicationID),
		ProtectionMode: wv1.ProtectionMode(p.Mode),
		DesiredState: &wv1.ProtectionDesiredState{
			ModeSec: &wv1.ModSec{
				ProtectionMode: wv1.ProtectionMode(p.DesiredState.ModSec.Mode),
				ParanoiaLevel:  wv1.ParanoiaLevel(p.DesiredState.ModSec.ParanoiaLevel),
			},
			XffNumTrustedHops: p.DesiredState.XffNumTrustedHops,
		},
	}
	if p.Application.ID != 0 {
		protection.Application = p.Application.ToProto()
	}
	for _, crsVersion := range p.CrsVersions {
		protection.CrsVersions = append(protection.CrsVersions, crsVersion.ToProto())
	}

	return protection
}

func (p *Protection) AfterCreate(tx *gorm.DB) error {
	crsRepo := NewCrsRepository(tx, nil)
	crsVersion := &CrsVersion{
		Tag:          DefaultCrsVersionTag,
		Status:       uint32(wv1.CrsVersionStatus_CRS_VERSION_STATUS_ACTIVE),
		ProtectionID: p.ID,
	}
	if err := crsRepo.CreateCrsVersion(crsVersion); err != nil {
		return err
	}

	return crsRepo.CloneCrsProfileToCrsRuleSet(DefaultCRSProfileName, crsVersion.ID)
}

func (s *ProtectionRepository) CreateProtection(req *wv1.CreateProtectionRequest) (*Protection, error) {
	protection := &Protection{
		ApplicationID: uint(req.ApplicationId),
		Mode:          uint32(req.ProtectionMode),
	}
	protection.DesiredState.FromProto(req.DesiredState)
	if err := s.db.Create(protection).Error; err != nil {
		return nil, err
	}
	return protection, nil
}

func (s *ProtectionRepository) GetProtection(id uint, options *wv1.GetProtectionOptions) (*Protection, error) {
	p := &Protection{}
	query := s.db.Model(&Protection{}).Where("id = ?", id)
	allRules := wv1.GetProtectionOptionsIncludeCrsRules_GET_PROTECTION_OPTIONS_INCLUDE_CRS_RULES_ALL
	activeRules := wv1.GetProtectionOptionsIncludeCrsRules_GET_PROTECTION_OPTIONS_INCLUDE_CRS_RULES_ACTIVE
	if options != nil && options.IncludeCrsRules != nil {
		if *options.IncludeCrsRules == allRules {
			query.Preload("CrsVersions.CrsRuleSets")
		}
		if *options.IncludeCrsRules == activeRules {
			query.Preload("CrsVersions", "status = ?", uint32(wv1.CrsVersionStatus_CRS_VERSION_STATUS_ACTIVE)).
				Preload("CrsVersions.CrsRuleSets", func(db *gorm.DB) *gorm.DB {
					return db.Order("id")
				})
		}
	}
	err := query.First(p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("protection not found"))
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return p, nil
}

func (s *ProtectionRepository) UpdateProtection(req *wv1.PutProtectionRequest) (*Protection, error) {
	protection := &Protection{ID: uint(req.GetId())}
	if req.ProtectionMode != nil {
		protection.Mode = uint32(*req.ProtectionMode)
	}
	if req.DesiredState != nil {
		desiredState := &ProtectionDesiredState{}
		desiredState.FromProto(req.DesiredState)
		protection.DesiredState = *desiredState
	}
	// fetch the application id for the given protection
	res := s.db.Model(&Protection{}).
		Select("application_id").
		Where("id = ?", protection.ID).
		Scan(&protection.ApplicationID)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) || res.RowsAffected == 0 {
			return nil, connect.NewError(connect.CodeNotFound, res.Error)
		}
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, res.Error)
	}
	res = s.db.
		Model(protection).
		Updates(protection)
	if res.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("protection id not found"))
	}

	return s.GetProtection(protection.ID, nil)
}

func (s *ProtectionRepository) ListProtections(options *wv1.ListProtectionsOptions) ([]*Protection, error) {
	if options == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("options are required"))
	}
	var err error
	var protections []*Protection
	query := s.db.Model(&Protection{})
	if options.ProtectionMode != nil {
		query = query.Where("protections.mode = ?", uint32(*options.ProtectionMode))
	}
	if options.ModSecMode != nil {
		query = query.Where(
			fmt.Sprintf(
				"protections.desired_state -> 'modSec' ->> 'protectionMode' = '%d'",
				uint32(*options.ModSecMode),
			),
		)
	}

	if options.IncludeApps != nil && *options.IncludeApps {
		err = query.
			Preload("Application.Ingresses.Upstream").
			Find(&protections).Error
		if err != nil {
			return protections, err
		}
		for i := 0; i < len(protections); i++ {
			for j := 0; j < len(protections[i].Application.Ingresses); j++ {
				if err := s.db.
					Where("upstream_id = ? and ingress_id = ?",
						protections[i].Application.Ingresses[j].UpstreamID,
						protections[i].Application.Ingresses[j].ID).
					Find(&protections[i].Application.Ingresses[j].Upstream.Ports).Error; err != nil {
					return protections, err
				}
			}
		}
	} else {
		err = query.Find(&protections).Error
	}
	return protections, err
}

func (s *ProtectionRepository) DeleteProtection(protectionId uint32) error {
	return s.db.Delete(&Protection{ID: uint(protectionId)}).Error
}
