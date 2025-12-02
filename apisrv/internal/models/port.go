package models

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Port struct {
	ID                 uint   `gorm:"primaryKey"`
	PortNumber         uint32 `gorm:"uniqueIndex:port_number_uid_iid_port_type;not null"`
	PortName           string
	Status             uint32
	ProxyListeningPort uint32
	PortType           uint32 `gorm:"uniqueIndex:port_number_uid_iid_port_type;not null"`
	Description        string
	UpstreamID         string   `gorm:"size:63;uniqueIndex:port_number_uid_iid_port_type;not null"`
	Upstream           Upstream `gorm:"foreignKey:UpstreamID;references:ID"`
	IngressID          uint     `gorm:"uniqueIndex:port_number_uid_iid_port_type;not null"`
	Ingress            Ingress
	CreatedAt          time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time `gorm:"default:CURRENT_TIMESTAMP"`
}

type PortRepository struct {
	db         *gorm.DB
	logger     *zap.Logger
	upstreamId string
	ingressId  uint
	Port       Port
}

func NewPortRepository(uId string, iId uint, tx *gorm.DB, logger *zap.Logger) *PortRepository {
	modelSvc := &PortRepository{
		db:         tx,
		logger:     logger,
		upstreamId: uId,
		ingressId:  iId,
	}
	if tx == nil {
		modelSvc.db = db()
	}
	if logger == nil {
		modelSvc.logger = applogger.NewLogger()
	}
	return modelSvc
}

func (s *PortRepository) Save(desiredPorts []Port) error {
	if len(desiredPorts) == 0 {
		return nil
	}
	// delete unexisting ports in case such exists
	if err := s.deleteUnexistingPorts(desiredPorts); err != nil {
		return err
	}
	assigmentColumns := []string{
		"port_name",
		"status",
		"proxy_listening_port",
		"port_type",
		"description",
		"upstream_id",
		"created_at",
		"updated_at",
	}
	for _, p := range desiredPorts {
		// set foreign upstream id
		p.UpstreamID = s.upstreamId
		// set foreign ingress id
		p.IngressID = s.ingressId
		if res := s.db.Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{Name: "port_number"},
					{Name: "upstream_id"},
					{Name: "ingress_id"},
					{Name: "port_type"},
				},
				DoUpdates: clause.AssignmentColumns(assigmentColumns),
			}).Create(&p); res.Error != nil {
			return connect.NewError(connect.CodeUnknown, res.Error)
		}
	}
	return nil
}

func (s *PortRepository) deleteUnexistingPorts(desiredPorts []Port) error {
	if len(desiredPorts) == 0 {
		return nil
	}
	// get current ports
	currentPorts, err := s.currentPorts()
	if err != nil {
		return err
	}
	// delete unexisting ports if case such exists
	for _, currentPort := range currentPorts {
		found := false
		for _, desiredPort := range desiredPorts {
			if currentPort.PortNumber == desiredPort.PortNumber {
				found = true
				break
			}
		}
		if !found {
			//if err := s.db.Delete(currentPort).Error; err != nil {
			//	return err
			//}
		}
	}
	return nil
}

func (s *PortRepository) currentPorts() (ports []*Port, err error) {
	query := s.db.Model(&Port{})
	query = query.
		//Joins("JOIN upstreams ON upstreams.id = ports.upstream_id").
		//Joins("JOIN ingresses ON ingresses.id = ports.ingress_id").
		Where("ingress_id = ?", s.ingressId)
	return ports, query.Distinct().Find(&ports).Error
}

func NewPortFromProto(port *wv1.Port) Port {
	return Port{
		PortNumber:         port.Number,
		PortName:           port.Name,
		ProxyListeningPort: port.ProxyListeningPort,
		Status:             uint32(port.Status),
		PortType:           uint32(port.PortType),
		Description:        port.Description,
	}
}

func NewPortsFromProto(port []*wv1.Port) (ports []Port) {
	ports = make([]Port, len(port))
	for idx, port := range port {
		ports[idx] = NewPortFromProto(port)
	}
	return ports
}

func (p *Port) ToProto() *wv1.Port {
	return &wv1.Port{
		Number:             p.PortNumber,
		Name:               p.PortName,
		ProxyListeningPort: p.ProxyListeningPort,
		Status:             wv1.PortStatusType(p.Status),
		PortType:           wv1.PortType(p.PortType),
		Description:        p.Description,
	}
}

func (p *Port) BeforeCreate(tx *gorm.DB) error {
	return p.allocateProxyListenerPort(tx)
}

func (p *Port) proxyProtAllocator(tx *gorm.DB) error {
	allocationAttempts := 10
	proxyListenerPort := func() int32 {
		rand.NewSource(time.Now().UnixNano())
		minPort := 49152
		maxPort := 65535
		return int32(uint32(rand.Intn(maxPort-minPort) + minPort))
	}()
	for allocationAttempts > 0 {
		query := tx.Where("proxy_listening_port = ?", proxyListenerPort)
		if err := query.First(&Port{}).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				p.ProxyListeningPort = uint32(proxyListenerPort)
				return nil
			}
			return err
		}
		allocationAttempts--
	}
	return fmt.Errorf("cound not find a free proxy listening port")
}

func (p *Port) allocateProxyListenerPort(tx *gorm.DB) error {
	// only container ports require proxy listeners ports allocations
	if p.PortType != uint32(wv1.PortType_PORT_TYPE_CONTAINER_PORT) {
		return nil
	}
	// get current port
	query := tx.Where("port_number = ? and upstream_id = ? and ingress_id = ? and port_type = ?",
		p.PortNumber,
		p.UpstreamID,
		p.IngressID,
		uint32(wv1.PortType_PORT_TYPE_CONTAINER_PORT))
	currentPort := &Port{}
	if err := query.First(currentPort).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return p.proxyProtAllocator(tx)
		} else {
			// return an error for any other than record not found error
			return err
		}
	}
	if currentPort.ProxyListeningPort == 0 {
		// allocate proxy port if its currently 0
		return p.proxyProtAllocator(tx)
	} else {
		// use the current port
		p.ProxyListeningPort = currentPort.ProxyListeningPort
	}
	// return here, since no allocation is needed
	return nil
}
