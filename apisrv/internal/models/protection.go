package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

type IP struct {
	CIDR string `json:"cidr"`
}
type IPRules struct {
	Block []IP `json:"block"`
	Allow []IP `json:"allow"`
}

type BasicAuthUser struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type TokenAuthToken struct {
	Token       string `json:"token"`
	ValidAfter  uint64 `json:"validAfter"`  // the earliest moment the token can be used.
	ValidBefore uint64 `json:"validBefore"` // the moment the token becomes "trash."
	Description string `json:"description"`
}

type BasicAuth struct {
	Users         []BasicAuthUser `json:"users"`
	Enabled       bool            `json:"enabled"`
	PathWhitelist []string        `json:"pathWhitelist"`
}

type TokenAuth struct {
	Header        string            `json:"header"`
	Tokens        []*TokenAuthToken `json:"tokens"`
	Enabled       bool              `json:"enabled"`
	PathWhitelist []string          `json:"pathWhitelist"`
}

type Auth struct {
	BasicAuth *BasicAuth `json:"basicAuth"`
	TokenAuth *TokenAuth `json:"tokenAuth"`
}
type ProtectionDesiredState struct {
	Waf               *Waf     `json:"waf"`
	XffNumTrustedHops *uint32  `json:"xffNumTrustedHops"`
	IPRules           *IPRules `json:"ipRules"`
	Auth              *Auth    `json:"auth"`
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

func (s *ProtectionDesiredState) Merge(req *wv1.PutProtectionRequest) {
	// nothing to merge if new desired state is nil
	if req == nil {
		return
	}
	// if new desired state waf is not nil, fully overwrite the Waf object
	if req.Waf != nil {
		s.Waf.FromProto(req.Waf)
	}
	// if XffNumTrustedHops set by request fully overwrite it
	if req.XffNumTrustedHops != nil {
		s.XffNumTrustedHops = req.XffNumTrustedHops
	}
	// if current IP Rules is empty, just set it accordingly to request
	if s.IPRules == nil {
		s.IPRules = &IPRules{}
	}
	s.IPRules.Merge(req)
	//set auth
	if s.Auth == nil {
		s.Auth = &Auth{}
	}
	// set basic auth
	if req.BasicAuth != nil {
		if s.Auth.BasicAuth == nil {
			s.Auth.BasicAuth = &BasicAuth{}
		}
		s.Auth.BasicAuth.Merge(req.BasicAuth)
	}
	// set token auth
	if req.TokenAuth != nil {
		if s.Auth.TokenAuth == nil {
			s.Auth.TokenAuth = &TokenAuth{}
		}
		s.Auth.TokenAuth.Merge(req.TokenAuth)
	}
}

func (ba *BasicAuth) Merge(reqBasicAuth *wv1.BasicAuthPutRequest) {
	// if set by request, set enabled
	if reqBasicAuth.Enabled != nil {
		ba.Enabled = *reqBasicAuth.Enabled
	}
	// find all indexes for removal
	var removeAt []int
	for idx, currentPath := range ba.PathWhitelist {
		for _, pathToRemove := range reqBasicAuth.PathWhitelistToRemove {
			if currentPath == pathToRemove {
				removeAt = append(removeAt, idx)
				continue
			}
		}
	}
	// remove paths whitelists
	for _, removeIdx := range removeAt {
		ba.PathWhitelist = append(ba.PathWhitelist[:removeIdx], ba.PathWhitelist[removeIdx+1:]...)
	}
	// add new paths whitelists
	for _, newPathWhitelist := range reqBasicAuth.PathWhitelistToAdd {
		found := false
		for _, currentPath := range ba.PathWhitelist {
			if newPathWhitelist == currentPath {
				found = true
			}
		}
		if !found {
			ba.PathWhitelist = append(ba.PathWhitelist, newPathWhitelist)
		}
	}
	// remove users
	removeAt = []int{}
	for idx, currentUser := range ba.Users {
		for _, userToRemove := range reqBasicAuth.UsersToRemove {
			if currentUser.User == userToRemove.User {
				removeAt = append(removeAt, idx)
			}
		}
	}
	for _, removeIdx := range removeAt {
		ba.Users = append(ba.Users[:removeIdx], ba.Users[removeIdx+1:]...)
	}
	// add users
	for _, userToAdd := range reqBasicAuth.UsersToAdd {
		found := false
		for _, currentUser := range ba.Users {
			if currentUser.User == userToAdd.User {
				// do not duplicate the user, but do update the password
				found = true
				if userToAdd.Pass != "" {
					currentUser.Pass = userToAdd.Pass
				}
			}
		}
		if !found {
			ba.Users = append(ba.Users, BasicAuthUser{User: userToAdd.User, Pass: userToAdd.Pass})
		}
	}
}

func (ta *TokenAuth) Merge(reqTokenAuth *wv1.TokenAuthPutRequest) {
	if reqTokenAuth == nil {
		return
	}
	// if set by request, set enabled
	if reqTokenAuth.Enabled != nil {
		ta.Enabled = *reqTokenAuth.Enabled
	}
	// find all indexes for removal
	var removeAt []int
	for idx, currentPath := range ta.PathWhitelist {
		for _, pathToRemove := range reqTokenAuth.PathWhitelistToRemove {
			if currentPath == pathToRemove {
				removeAt = append(removeAt, idx)
				continue
			}
		}
	}
	// remove paths whitelists
	for _, removeIdx := range removeAt {
		ta.PathWhitelist = append(ta.PathWhitelist[:removeIdx], ta.PathWhitelist[removeIdx+1:]...)
	}
	// add new paths whitelists
	for _, newPathWhitelist := range reqTokenAuth.PathWhitelistToAdd {
		found := false
		for _, currentPath := range ta.PathWhitelist {
			if newPathWhitelist == currentPath {
				found = true
			}
		}
		if !found {
			ta.PathWhitelist = append(ta.PathWhitelist, newPathWhitelist)
		}
	}
	// remove tokens
	removeAt = []int{}
	for idx, currentToken := range ta.Tokens {
		for _, tokenToRemove := range reqTokenAuth.TokensToRemove {
			if currentToken.Token == tokenToRemove.Token {
				removeAt = append(removeAt, idx)
			}
		}
	}
	for _, removeIdx := range removeAt {
		ta.Tokens = append(ta.Tokens[:removeIdx], ta.Tokens[removeIdx+1:]...)
	}
	// add tokens
	for _, tokenToAdd := range reqTokenAuth.TokensToAdd {
		found := false
		for _, currentToken := range ta.Tokens {
			if currentToken.Token == tokenToAdd.Token {
				// do not duplicate the token, but do update the fields
				found = true
				if tokenToAdd.ValidAfter != nil {
					currentToken.ValidAfter = *tokenToAdd.ValidAfter
				}
				if tokenToAdd.ValidBefore != nil {
					currentToken.ValidBefore = *tokenToAdd.ValidBefore
				}
				if tokenToAdd.Description != nil {
					currentToken.Description = *tokenToAdd.Description
				}

			}
		}
		if !found {
			token := &TokenAuthToken{Token: tokenToAdd.Token}
			if tokenToAdd.ValidAfter != nil {
				token.ValidAfter = *tokenToAdd.ValidAfter
			}
			if tokenToAdd.ValidBefore != nil {
				token.ValidBefore = *tokenToAdd.ValidBefore
			}
			if tokenToAdd.Description != nil {
				token.Description = *tokenToAdd.Description
			}
			ta.Tokens = append(ta.Tokens, token)
		}
	}
}

func (p *IPRules) Merge(putProtectionReq *wv1.PutProtectionRequest) {
	var block []*wv1.IP
	var allow []*wv1.IP
	// remove IPs
	if putProtectionReq.IpRulesToRemove != nil {
		// remove from allow list
		allow = append(allow, removeIPs(p.Allow, putProtectionReq.IpRulesToRemove.Allow)...)
		// remove from block list
		block = append(block, removeIPs(p.Block, putProtectionReq.IpRulesToRemove.Block)...)
	}
	// add IPs
	if putProtectionReq.IpRulesToAdd != nil {
		//add to allow list
		allow = append(allow, addIPs(p.Allow, putProtectionReq.IpRulesToAdd.Allow)...)
		// add to block list
		block = append(block, addIPs(p.Block, putProtectionReq.IpRulesToAdd.Block)...)
	}
	p.FromProto(&wv1.IPRules{Allow: allow, Block: block})
}

func (p *IPRules) Validate() error {
	for _, rule := range p.Block {
		if rule.CIDR == "" {
			return fmt.Errorf("CIDR cannot be empty")
		}
		_, _, err := net.ParseCIDR(rule.CIDR)
		if err != nil {
			return fmt.Errorf("invalid CIDR format '%s': %v", rule.CIDR, err)
		}
	}
	return nil
}

func (p *IPRules) FromProto(ipRules *wv1.IPRules) {
	// allowed IPs
	p.Allow = make([]IP, len(ipRules.Allow))
	for i, ip := range ipRules.Allow {
		p.Allow[i] = IP{CIDR: ip.Cidr}
	}
	// blocked IPs
	p.Block = make([]IP, len(ipRules.Block))
	for i, ip := range ipRules.Block {
		p.Block[i] = IP{CIDR: ip.Cidr}
	}
}

func (f *Waf) FromProto(v1desiredState *wv1.Waf) {
	f.ParanoiaLevel = uint32(v1desiredState.ParanoiaLevel)
	f.Mode = uint32(v1desiredState.ProtectionMode)
}

func (s *ProtectionDesiredState) ToProto() *wv1.ProtectionDesiredState {
	state := &wv1.ProtectionDesiredState{
		Auth: &wv1.Auth{BasicAuth: &wv1.BasicAuth{}, TokenAuth: &wv1.TokenAuth{}}}
	// waf
	if s.Waf != nil {
		state.Waf = &wv1.Waf{
			ProtectionMode: wv1.ProtectionMode(s.Waf.Mode),
			ParanoiaLevel:  wv1.ParanoiaLevel(s.Waf.ParanoiaLevel),
		}
	}
	// trusted hops
	if s.XffNumTrustedHops != nil {
		state.XffNumTrustedHops = s.XffNumTrustedHops
	}
	// ip rules
	if s.IPRules != nil {
		var block = make([]*wv1.IP, len(s.IPRules.Block))
		for idx, ip := range s.IPRules.Block {
			block[idx] = &wv1.IP{Cidr: ip.CIDR}
		}
		var allow = make([]*wv1.IP, len(s.IPRules.Allow))
		for idx, ip := range s.IPRules.Allow {
			allow[idx] = &wv1.IP{Cidr: ip.CIDR}
		}
		state.IpRules = &wv1.IPRules{Allow: allow, Block: block}
	}
	if s.Auth != nil {
		// basic auth
		if s.Auth.BasicAuth != nil {
			basicAuthUser := make([]*wv1.BasicAuthUser, len(s.Auth.BasicAuth.Users))
			for idx, user := range s.Auth.BasicAuth.Users {
				basicAuthUser[idx] = &wv1.BasicAuthUser{User: user.User, Pass: user.Pass}
			}
			state.Auth.BasicAuth = &wv1.BasicAuth{
				Users:         basicAuthUser,
				PathWhitelist: s.Auth.BasicAuth.PathWhitelist,
				Enabled:       &s.Auth.BasicAuth.Enabled,
			}
		}
		// token auth
		if s.Auth.TokenAuth != nil {
			authTokens := make([]*wv1.TokenAuthToken, len(s.Auth.TokenAuth.Tokens))
			for idx, token := range s.Auth.TokenAuth.Tokens {
				authTokens[idx] = &wv1.TokenAuthToken{
					Token:       token.Token,
					ValidAfter:  &token.ValidAfter,
					ValidBefore: &token.ValidBefore,
					Description: &token.Description,
				}
			}
			state.Auth.TokenAuth = &wv1.TokenAuth{
				Header:        s.Auth.TokenAuth.Header,
				Tokens:        authTokens,
				PathWhitelist: s.Auth.TokenAuth.PathWhitelist,
				Enabled:       &s.Auth.TokenAuth.Enabled,
			}
		}
	}
	return state
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
		DesiredState:   p.DesiredState.ToProto(),
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
	// init new Protection model
	protection := &Protection{ID: uint(req.GetId())}
	// set current protection mode, application id and current desired state
	if err := s.db.Raw("SELECT mode, application_id, desired_state FROM protections WHERE id = ?", protection.ID).
		Row().
		Scan(&protection.Mode, &protection.ApplicationID, &protection.DesiredState); err != nil {
		s.logger.Error(err.Error())
		return nil, connect.NewError(connect.CodeNotFound, errors.New("protection not found"))
	}
	if protection.ApplicationID == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("protection not found"))
	}
	// overwrite protection mode if set by request
	if req.ProtectionMode != nil {
		protection.Mode = uint32(*req.ProtectionMode)
	}
	protection.DesiredState.Merge(req)
	// validate IP rules are correct
	if err := protection.DesiredState.IPRules.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
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

// addIPs add IPs without duplications
func addIPs(current []IP, fromReq []*wv1.IP) (resultIps []*wv1.IP) {
	// add existing IPs without duplicates
	for _, currentIP := range current {
		found := false
		for _, reqIP := range fromReq {
			if currentIP.CIDR == reqIP.Cidr {
				found = true
			}
		}
		if !found {
			resultIps = append(resultIps, &wv1.IP{Cidr: currentIP.CIDR})
		}
	}
	// add new IPs
	for _, reqIP := range fromReq {
		found := false
		for _, currentIP := range resultIps {
			if currentIP.Cidr == reqIP.Cidr {
				found = true
			}
		}
		if !found {
			resultIps = append(resultIps, reqIP)
		}
	}
	return resultIps
}

// removeIPs remove ips
func removeIPs(current []IP, fromReq []*wv1.IP) (resultIps []*wv1.IP) {
	for _, currentIP := range current {
		found := false
		for _, fromReqIp := range fromReq {
			if currentIP.CIDR == fromReqIp.Cidr {
				found = true
			}
		}
		if !found {
			resultIps = append(resultIps, &wv1.IP{Cidr: currentIP.CIDR})
		}
	}
	return resultIps
}
