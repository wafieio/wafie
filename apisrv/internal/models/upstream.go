package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

type Endpoint struct {
	NodeName  string `json:"nodeName"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func NewEndpointFromRequest(ep *wv1.Endpoint) Endpoint {
	return Endpoint{
		NodeName:  ep.NodeName,
		Kind:      ep.Kind,
		Name:      ep.Name,
		Namespace: ep.Namespace,
	}
}

func (e *Endpoint) ToProto(ip string) *wv1.Endpoint {
	return &wv1.Endpoint{
		Ip:        ip,
		NodeName:  e.NodeName,
		Kind:      e.Kind,
		Name:      e.Name,
		Namespace: e.Namespace,
	}
}

func NewMirrorPolicyFromRequest(mp *wv1.MirrorPolicy) *MirrorPolicy {
	if mp == nil {
		return nil
	}
	return &MirrorPolicy{
		Status: uint32(mp.Status),
		Ip:     mp.Ip,
		Port:   int(mp.Port),
		Dns:    mp.Dns,
	}
}

func (p *MirrorPolicy) ToProto() *wv1.MirrorPolicy {
	if p == nil {
		return nil
	}
	return &wv1.MirrorPolicy{
		Status: wv1.MirrorPolicyStatus(p.Status),
		Ip:     p.Ip,
		Port:   uint32(p.Port),
		Dns:    p.Dns,
	}
}

func (p *MirrorPolicy) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, p)
	case string:
		return json.Unmarshal([]byte(v), p)
	default:
		return fmt.Errorf("unsupported type for ProtectionDesiredState")
	}
}

func (p *MirrorPolicy) Value() (driver.Value, error) {
	return json.Marshal(p)
}

type Endpoints map[string]Endpoint

func (e Endpoints) Value() (driver.Value, error) {
	if e == nil {
		return nil, nil
	}
	jsonData, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Endpoints to JSON: %w", err)
	}
	return string(jsonData), nil
}

func (e *Endpoints) Scan(value interface{}) error {
	if value == nil {
		*e = nil
		return nil
	}
	var jsonData []byte
	switch v := value.(type) {
	case []byte:
		jsonData = v
	case string:
		jsonData = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into Endpoints", value)
	}
	if *e == nil {
		*e = make(Endpoints)
	}
	return json.Unmarshal(jsonData, e)
}

type MirrorPolicy struct {
	Status uint32 `json:"status"`
	Ip     string `json:"ip"`
	Port   int    `json:"port"`
	Dns    string `json:"dns"`
}

type Upstream struct {
	ID                string        `gorm:"primaryKey;size:63"` // svc fqdn acting as a primary key for Upstream
	Endpoints         *Endpoints    `gorm:"type:jsonb"`
	MirrorPolicy      *MirrorPolicy `gorm:"type:jsonb"`
	UpstreamRouteType uint32
	Ingresses         []Ingress `gorm:"foreignKey:UpstreamID"`
	Ports             []Port    `gorm:"foreignKey:UpstreamID"`
	CreatedAt         time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type UpstreamRepository struct {
	db       *gorm.DB
	logger   *zap.Logger
	Upstream Upstream
}

func NewUpstreamRepository(tx *gorm.DB, logger *zap.Logger) *UpstreamRepository {
	modelSvc := &UpstreamRepository{db: tx, logger: logger}
	if tx == nil {
		modelSvc.db = db()
	}
	if logger == nil {
		modelSvc.logger = applogger.NewLogger()
	}
	return modelSvc
}

func NewUpstreamFromRequest(upstreamReq *wv1.Upstream) *Upstream {
	mirrorPolicy := NewMirrorPolicyFromRequest(upstreamReq.MirrorPolicy)
	u := &Upstream{
		ID:                upstreamReq.SvcFqdn,
		UpstreamRouteType: uint32(upstreamReq.UpstreamRouteType),
		MirrorPolicy:      mirrorPolicy,
	}
	if upstreamReq.Endpoints != nil {
		eps := make(Endpoints, len(upstreamReq.Endpoints))
		u.Endpoints = &eps
		for _, ep := range upstreamReq.Endpoints {
			(*u.Endpoints)[ep.Ip] = NewEndpointFromRequest(ep)
		}
	}
	return u
}

func (s *UpstreamRepository) Save(u *Upstream) (*Upstream, error) {
	omitColumns := []string{"Ingresses", "Ports"}
	if u.MirrorPolicy == nil {
		omitColumns = append(omitColumns, "mirror_policy")
	}
	if u.Endpoints == nil {
		omitColumns = append(omitColumns, "endpoints")
	}
	return u, s.db.
		Omit(omitColumns...).
		Save(&u).Error
}

func (s *UpstreamRepository) List(options *wv1.ListRoutesOptions) (upstreams []*Upstream, err error) {
	query := s.db.Model(&Upstream{})
	if options != nil && options.IncludeIngress != nil && *options.IncludeIngress {
		query = query.
			Joins("JOIN ingresses ON ingresses.upstream_id = upstreams.id").
			Joins("JOIN ports ON ports.upstream_id = upstreams.id").
			Preload("Ingresses").
			Preload("Ports")
	}
	if options != nil && options.SvcFqdn != nil {
		query = query.Where("svc_fqdn = ?", options.SvcFqdn)
	}
	return upstreams, query.Distinct().Find(&upstreams).Error

}

func (u *Upstream) ToProto() *wv1.Upstream {
	wv1upstream := &wv1.Upstream{
		SvcFqdn:           u.ID,
		UpstreamRouteType: wv1.UpstreamRouteType(u.UpstreamRouteType),
		MirrorPolicy:      u.MirrorPolicy.ToProto(),
	}
	if u.Endpoints != nil {
		for ip, ep := range *u.Endpoints {
			wv1upstream.Endpoints = append(wv1upstream.Endpoints, ep.ToProto(ip))
		}
	}

	if u.Ports != nil {
		wv1upstream.Ports = make([]*wv1.Port, len(u.Ports))
		for idx, port := range u.Ports {
			wv1upstream.Ports[idx] = port.ToProto()
		}
	}
	return wv1upstream
}

func (u *Upstream) BeforeSave(tx *gorm.DB) (err error) {

	// set default upstream route type
	if u.UpstreamRouteType == uint32(wv1.UpstreamRouteType_UPSTREAM_ROUTE_TYPE_UNSPECIFIED) {
		u.UpstreamRouteType = uint32(wv1.UpstreamRouteType_UPSTREAM_ROUTE_TYPE_PORT)
	}
	return nil
}
