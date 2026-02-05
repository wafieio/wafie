package models

import (
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ApplicationRepository struct {
	db          *gorm.DB
	logger      *zap.Logger
	Application Application
}

type Application struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"uniqueIndex:idx_application_name"`
	Ingresses []Ingress `gorm:"foreignKey:ApplicationID;references:ID"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewApplicationRepository(tx *gorm.DB, logger *zap.Logger) *ApplicationRepository {
	modelSvc := &ApplicationRepository{db: tx, logger: logger}
	if tx == nil {
		modelSvc.db = db()
	}
	if logger == nil {
		modelSvc.logger = applogger.NewLogger()
	}
	return modelSvc
}

func (a *Application) FromProto(req *v1.CreateApplicationRequest) error {
	if req.Name == "" {
		return errors.New("name and namespace are required")
	}
	a.Name = req.Name
	return nil
}

func (a *Application) ToProto() *v1.Application {
	applicationIngresses := make([]*v1.Ingress, len(a.Ingresses))
	for idx, ingress := range a.Ingresses {
		applicationIngresses[idx] = ingress.ToProto()
	}
	return &v1.Application{
		Id:      uint32(a.ID),
		Name:    a.Name,
		Ingress: applicationIngresses,
	}
}

func (s *ApplicationRepository) GetApplication(req *v1.GetApplicationRequest) (*Application, error) {
	app := &Application{ID: uint(req.GetId())}
	err := s.db.Preload("Ingresses").
		Preload("Ingresses.Upstream").
		First(&app, req.GetId()).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("application not found"))
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return app, nil
}

func (s *ApplicationRepository) CreateApplication(req *v1.CreateApplicationRequest) (*Application, error) {
	app := &Application{}
	if err := app.FromProto(req); err != nil {
		return app, err
	}

	if err := s.db.Create(app).Error; err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	return app, nil
}

func (s *ApplicationRepository) ListApplications(options *v1.ListApplicationsOptions) ([]*Application, error) {
	var err error
	var apps []*Application
	if options.IncludeIngress {
		err = s.db.Preload("Ingresses").
			Preload("Ingresses.Upstream").
			Find(&apps).Error
	} else {
		err = s.db.Find(&apps).Error
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}
	return apps, nil
}

func (s *ApplicationRepository) UpdateApplication(req *v1.Application) (*Application, error) {
	var app Application
	// Prevent changing immutable fields
	if req.GetName() != "" && app.Name != req.GetName() {
		return nil, errors.New("cannot change application name")
	}
	return &app, nil
	//
	//// Load app and protections
	//if err := s.dbPreload("Protections.WAFConfig").First(&app, req.GetId()).Error; err != nil {
	//	if errors.Is(err, gorm.ErrRecordNotFound) {
	//		return nil, connect.NewError(connect.CodeNotFound, errors.New("application not found"))
	//	}
	//	return nil, connect.NewError(connect.CodeInternal, err)
	//}
	//
	//// Build map of existing protections by type
	//existingByType := make(map[ProtectionType]*Protection)
	//for i := range app.Protections {
	//	existingByType[app.Protections[i].Type] = &app.Protections[i]
	//}
	//
	//// Track types sent in update
	//seenTypes := make(map[ProtectionType]bool)
	//
	//for _, update := range req.GetProtections() {
	//	if update.Status == nil {
	//		return nil, errors.New("protection status is required")
	//	}
	//
	//	// Determine type based on config
	//	var pType ProtectionType
	//	switch update.Config.(type) {
	//	case *v1.Protection_Waf:
	//		pType = ProtectionTypeWAF
	//	default:
	//		return nil, fmt.Errorf("unsupported or missing protection config")
	//	}
	//
	//	seenTypes[pType] = true
	//
	//	// Update existing or create new
	//	existing := existingByType[pType]
	//	if existing != nil {
	//		// Handle updates
	//		configChanged := false
	//		switch cfg := update.Config.(type) {
	//		case *v1.Protection_Waf:
	//			if existing.WAFConfig == nil {
	//				existing.WAFConfig = &ModSecProtectionConfig{}
	//				configChanged = true
	//			}
	//			if existing.WAFConfig.RuleSet != cfg.Waf.RuleSet {
	//				existing.WAFConfig.RuleSet = cfg.Waf.RuleSet
	//				configChanged = true
	//			}
	//			if !reflect.DeepEqual(existing.WAFConfig.AllowListIPs, cfg.Waf.AllowListIps) {
	//				existing.WAFConfig.AllowListIPs = cfg.Waf.AllowListIps
	//				configChanged = true
	//			}
	//		}
	//
	//		if existing.DesiredState != ProtectionState(update.Status.Desired.String()) {
	//			configChanged = true
	//		}
	//
	//		if configChanged {
	//			existing.ActualState = ProtectionUnspecified
	//			existing.LastUpdated = time.Now()
	//			existing.Reason = "protection updated"
	//		}
	//
	//		existing.DesiredState = ProtectionState(update.Status.Desired.String())
	//		existing.Reason = update.Status.Reason
	//
	//	} else {
	//		// Add new protection
	//		newProtection := Protection{
	//			ApplicationID: app.ID,
	//			Type:          pType,
	//			DesiredState:  ProtectionState(update.Status.Desired.String()),
	//			ActualState:   ProtectionUnspecified,
	//			LastUpdated:   time.Now(),
	//			Reason:        update.Status.Reason,
	//		}
	//
	//		switch cfg := update.Config.(type) {
	//		case *v1.Protection_Waf:
	//			newProtection.WAFConfig = &ModSecProtectionConfig{
	//				RuleSet:      cfg.Waf.RuleSet,
	//				AllowListIPs: cfg.Waf.AllowListIps,
	//			}
	//		}
	//
	//		app.Protections = append(app.Protections, newProtection)
	//	}
	//}
	//
	//// Delete any protections not included in update
	//for _, existing := range app.Protections {
	//	if !seenTypes[existing.Type] {
	//		if err := s.dbWhere("protection_id = ?", existing.ID).Delete(&ModSecProtectionConfig{}).Error; err != nil {
	//			return nil, fmt.Errorf("failed to delete WAF config for protection %d: %w", existing.ID, err)
	//		}
	//		if err := s.dbDelete(&existing).Error; err != nil {
	//			return nil, fmt.Errorf("failed to delete protection %d: %w", existing.ID, err)
	//		}
	//	}
	//}
	//
	//// Save updated app
	//if err := s.dbSession(&gorm.Session{FullSaveAssociations: true}).Save(&app).Error; err != nil {
	//	return nil, connect.NewError(connect.CodeInternal, err)
	//}
	//
	//return &app, nil
}
