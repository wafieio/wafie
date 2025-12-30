package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"connectrpc.com/connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type EventRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// EventData represents the JSONB data column from FluentBit
type EventData map[string]interface{}

func (e *EventData) Scan(value interface{}) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, e)
	case string:
		return json.Unmarshal([]byte(v), e)
	default:
		return fmt.Errorf("unsupported type for EventData")
	}
}

func (e EventData) Value() (driver.Value, error) {
	return json.Marshal(e)
}

// Event stores WAF security events from FluentBit
// Note: This table is created by FluentBit pgsql plugin, not GORM
// Schema: tag TEXT, time TIMESTAMP, data JSONB (no primary key)
type Event struct {
	Tag  string    `gorm:"column:tag"`
	Time time.Time `gorm:"column:time;index:idx_events_time,sort:desc"`
	Data EventData `gorm:"column:data;type:jsonb"`
}

// TableName overrides the default table name
func (Event) TableName() string {
	return "events"
}

func NewEventRepository(tx *gorm.DB, logger *zap.Logger) *EventRepository {
	repo := &EventRepository{db: tx, logger: logger}
	if tx == nil {
		repo.db = db()
	}
	if logger == nil {
		repo.logger = applogger.NewLogger()
	}
	return repo
}

func (e *Event) ToProto() *wv1.Event {
	data, _ := structpb.NewStruct(e.Data)
	return &wv1.Event{
		Tag:  e.Tag,
		Time: timestamppb.New(e.Time),
		Data: data,
	}
}

// ListEvents retrieves events with filtering options
func (r *EventRepository) ListEvents(req *wv1.ListEventsRequest) ([]*Event, int64, error) {
	var events []*Event
	var total int64

	query := r.db.Model(&Event{})

	// Apply time range filter
	if req.TimeRange != nil {
		if req.TimeRange.Start != nil {
			query = query.Where("time >= ?", req.TimeRange.Start.AsTime())
		}
		if req.TimeRange.End != nil {
			query = query.Where("time <= ?", req.TimeRange.End.AsTime())
		}
	}

	// Apply protection_id filter (from JSONB path)
	if req.ProtectionId != nil {
		query = query.Where(
			"data->'transaction'->'request'->'headers'->>'x-wafie-protection-id' = ?",
			fmt.Sprintf("%d", *req.ProtectionId),
		)
	}

	// Apply severity filter
	if req.Severity != nil {
		query = query.Where(
			"data->'transaction'->'messages'->0->'details'->>'severity' = ?",
			fmt.Sprintf("%d", *req.Severity),
		)
	}

	// Apply rule_id filter
	if req.RuleId != nil {
		query = query.Where(
			"data->'transaction'->'messages'->0->'details'->>'ruleId' = ?",
			*req.RuleId,
		)
	}

	// Apply client_ip filter
	if req.ClientIp != nil {
		query = query.Where(
			"data->'transaction'->>'client_ip' = ?",
			*req.ClientIp,
		)
	}

	// Get total count before pagination
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	// Apply ordering
	query = query.Order("time DESC")

	// Apply pagination
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	offset := int(req.GetOffset())
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&events).Error; err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	return events, total, nil
}

// GetEventStats retrieves aggregated statistics
func (r *EventRepository) GetEventStats(req *wv1.GetEventStatsRequest) (*wv1.EventStats, error) {
	// Determine time interval
	interval := "24 hours"
	if req.TimeRange != nil && req.TimeRange.Start != nil && req.TimeRange.End != nil {
		duration := req.TimeRange.End.AsTime().Sub(req.TimeRange.Start.AsTime())
		interval = fmt.Sprintf("%d seconds", int(duration.Seconds()))
	}

	// Build base query with time filter
	baseWhere := fmt.Sprintf("time > NOW() - INTERVAL '%s'", interval)
	if req.TimeRange != nil {
		if req.TimeRange.Start != nil {
			baseWhere = fmt.Sprintf("time >= '%s'", req.TimeRange.Start.AsTime().Format(time.RFC3339))
		}
		if req.TimeRange.End != nil {
			baseWhere = fmt.Sprintf("%s AND time <= '%s'", baseWhere, req.TimeRange.End.AsTime().Format(time.RFC3339))
		}
	}

	// Add protection_id filter if specified
	if req.ProtectionId != nil {
		baseWhere = fmt.Sprintf("%s AND data->'transaction'->'request'->'headers'->>'x-wafie-protection-id' = '%d'",
			baseWhere, *req.ProtectionId)
	}

	stats := &wv1.EventStats{}

	// Get total events and blocked count
	var result struct {
		TotalEvents    int64 `gorm:"column:total_events"`
		BlockedAttacks int64 `gorm:"column:blocked_attacks"`
		UniqueIPs      int64 `gorm:"column:unique_ips"`
	}
	r.db.Raw(fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_events,
			COUNT(*) FILTER (
				WHERE data @> '{"transaction":{"messages":[{"details":{"ruleId":"949110"}}]}}'
			) AS blocked_attacks,
			COUNT(DISTINCT data->'transaction'->>'client_ip') AS unique_ips
		FROM events
		WHERE %s
	`, baseWhere)).Scan(&result)

	stats.TotalEvents = result.TotalEvents
	stats.BlockedAttacks = result.BlockedAttacks
	stats.UniqueIps = result.UniqueIPs

	// Get severity breakdown
	var severityCounts []struct {
		Severity string `gorm:"column:severity"`
		Count    int64  `gorm:"column:count"`
	}
	r.db.Raw(fmt.Sprintf(`
		SELECT
			data->'transaction'->'messages'->0->'details'->>'severity' AS severity,
			COUNT(*) AS count
		FROM events
		WHERE %s
		GROUP BY severity
		ORDER BY count DESC
	`, baseWhere)).Scan(&severityCounts)

	for _, sc := range severityCounts {
		if sc.Severity != "" {
			stats.BySeverity = append(stats.BySeverity, &wv1.SeverityCount{
				Severity: sc.Severity,
				Count:    sc.Count,
			})
		}
	}

	// Get top rules
	var topRules []struct {
		RuleId string `gorm:"column:rule_id"`
		Count  int64  `gorm:"column:count"`
	}
	r.db.Raw(fmt.Sprintf(`
		SELECT
			data->'transaction'->'messages'->0->'details'->>'ruleId' AS rule_id,
			COUNT(*) AS count
		FROM events
		WHERE %s AND data->'transaction'->'messages'->0->'details'->>'ruleId' IS NOT NULL
		GROUP BY rule_id
		ORDER BY count DESC
		LIMIT 10
	`, baseWhere)).Scan(&topRules)

	for _, tr := range topRules {
		stats.TopRules = append(stats.TopRules, &wv1.RuleCount{
			RuleId: tr.RuleId,
			Count:  tr.Count,
		})
	}

	// Get top IPs
	var topIPs []struct {
		IP    string `gorm:"column:ip"`
		Count int64  `gorm:"column:count"`
	}
	r.db.Raw(fmt.Sprintf(`
		SELECT
			data->'transaction'->>'client_ip' AS ip,
			COUNT(*) AS count
		FROM events
		WHERE %s AND data->'transaction'->>'client_ip' IS NOT NULL
		GROUP BY ip
		ORDER BY count DESC
		LIMIT 10
	`, baseWhere)).Scan(&topIPs)

	for _, tip := range topIPs {
		stats.TopIps = append(stats.TopIps, &wv1.IpCount{
			Ip:    tip.IP,
			Count: tip.Count,
		})
	}

	// Get per-protection stats if requested
	if req.IncludePerProtection != nil && *req.IncludePerProtection {
		var protectionStats []struct {
			ProtectionID   string `gorm:"column:protection_id"`
			TotalEvents    int64  `gorm:"column:total_events"`
			BlockedAttacks int64  `gorm:"column:blocked_attacks"`
		}
		r.db.Raw(fmt.Sprintf(`
			SELECT
				data->'transaction'->'request'->'headers'->>'x-wafie-protection-id' AS protection_id,
				COUNT(*) AS total_events,
				COUNT(*) FILTER (
					WHERE data @> '{"transaction":{"messages":[{"details":{"ruleId":"949110"}}]}}'
				) AS blocked_attacks
			FROM events
			WHERE %s AND data->'transaction'->'request'->'headers'->>'x-wafie-protection-id' IS NOT NULL
			GROUP BY protection_id
			ORDER BY total_events DESC
		`, baseWhere)).Scan(&protectionStats)

		for _, ps := range protectionStats {
			stats.PerProtection = append(stats.PerProtection, &wv1.ProtectionStats{
				ProtectionId:   ps.ProtectionID,
				TotalEvents:    ps.TotalEvents,
				BlockedAttacks: ps.BlockedAttacks,
			})
		}
	}

	return stats, nil
}
