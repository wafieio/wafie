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

type Waf struct {
	Mode          uint32 `json:"protectionMode"`
	ParanoiaLevel uint32 `json:"paranoiaLevel"`
}

type IPBlockRule struct {
	CIDR string `json:"cidr,omitempty"`
}
type IPRules struct {
	IPBlockRules []IPBlockRule `json:"ipBlockRules,omitempty"`
}
type ProtectionDesiredState struct {
	Waf               *Waf     `json:"waf,omitempty"`
	XffNumTrustedHops *uint32  `json:"xffNumTrustedHops,omitempty"`
	IPRules           *IPRules `json:"ipRules,omitempty"`
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
	if v1desiredState == nil {
		return
	}
	if v1desiredState.Waf != nil {
		if s.Waf == nil {
			s.Waf = &Waf{}
		}
		s.Waf.FromProto(v1desiredState.Waf)
	}
	if v1desiredState.XffNumTrustedHops != nil {
		s.XffNumTrustedHops = v1desiredState.XffNumTrustedHops
	}
	if v1desiredState.IpRules != nil {
		if s.IPRules == nil {
			s.IPRules = &IPRules{}
		}
		s.IPRules.FromProto(v1desiredState.IpRules)
	}
}

func (s *ProtectionDesiredState) Merge(newDesiredState *ProtectionDesiredState) {
	// nothing to merge if new desired state is nil
	if newDesiredState == nil {
		return
	}
	// if new desired state waf is not nil, fully overwrite the Waf object
	if newDesiredState.Waf != nil {
		s.Waf = newDesiredState.Waf
	}
	// if XffNumTrustedHops set by request fully overwrite it
	if newDesiredState.XffNumTrustedHops != nil {
		s.XffNumTrustedHops = newDesiredState.XffNumTrustedHops
	}
	// if IPRules was set by the request
	if newDesiredState.IPRules != nil {
		// if current protection has no IPRules, set it
		if s.IPRules == nil {
			s.IPRules = &IPRules{IPBlockRules: newDesiredState.IPRules.IPBlockRules}
			// fully overwrite ip block rules in case they were fully empty
		} else if len(newDesiredState.IPRules.IPBlockRules) == 0 {
			s.IPRules.IPBlockRules = newDesiredState.IPRules.IPBlockRules
		} else {
			// the ip block rules already present, make sure we've no duplicates
			for _, newIpBlockRule := range newDesiredState.IPRules.IPBlockRules {
				newIpRuleFound := false
				for _, ipBlockRule := range s.IPRules.IPBlockRules {
					if newIpBlockRule == ipBlockRule {
						newIpRuleFound = true
					}
				}
				if !newIpRuleFound {
					s.IPRules.IPBlockRules = append(s.IPRules.IPBlockRules, newIpBlockRule)
				}
			}
		}
	}
}

func (f *Waf) FromProto(v1desiredState *wv1.Waf) {
	f.ParanoiaLevel = uint32(v1desiredState.ParanoiaLevel)
	f.Mode = uint32(v1desiredState.ProtectionMode)
}

func (p *IPRules) FromProto(ipRules *wv1.IPRules) {
	p.IPBlockRules = make([]IPBlockRule, len(ipRules.IpBlockRules))
	for i, ipBlockRule := range ipRules.IpBlockRules {
		p.IPBlockRules[i] = IPBlockRule{CIDR: ipBlockRule.Cidr}
	}
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
			Waf: &wv1.Waf{
				ProtectionMode: wv1.ProtectionMode(p.DesiredState.Waf.Mode),
				ParanoiaLevel:  wv1.ParanoiaLevel(p.DesiredState.Waf.ParanoiaLevel),
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
	// prepare new desired state
	var newDesiredState *ProtectionDesiredState
	if req.DesiredState != nil {
		newDesiredState = &ProtectionDesiredState{}
		newDesiredState.FromProto(req.DesiredState)
	}
	// init new Protection model
	protection := &Protection{ID: uint(req.GetId())}
	// set default protection mode if not set by request
	if req.ProtectionMode != nil {
		protection.Mode = uint32(*req.ProtectionMode)
	}
	// set application id and current desired state
	if err := s.db.Raw("SELECT application_id, desired_state FROM protections WHERE id = ?", protection.ID).
		Row().
		Scan(&protection.ApplicationID, &protection.DesiredState); err != nil {
		s.logger.Error(err.Error())
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if protection.ApplicationID == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("protection not found"))
	}
	protection.DesiredState.Merge(newDesiredState)
	res := s.db.
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
