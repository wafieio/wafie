package models

import (
	"fmt"
	"github.com/wafieio/wafie/apisrv/internal/models/assets/modsec"
	"github.com/wafieio/wafie/apisrv/internal/models/assets/sql"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	dbConn   *gorm.DB
	logger   *zap.Logger
	migrated bool
	seeded   bool
)

type DbCfg struct {
	host     string
	port     int
	user     string
	password string
	dbName   string
}

func NewDbCfg(host string, port int, user, pass, dbName string, log *zap.Logger) *DbCfg {
	logger = log
	return &DbCfg{
		host:     host,
		port:     port,
		user:     user,
		password: pass,
		dbName:   dbName,
	}
}

func (c *DbCfg) dsn() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		c.host, c.port, c.user, c.password, c.dbName)
}

func NewDb(cfg *DbCfg) (*gorm.DB, error) {
	if dbConn != nil {
		logger.Info("dbConn connection already established, reusing connection")
		return dbConn, nil
	}
	logger.Info("initiating db connection")
	var err error
	dbConn, err = gorm.Open(postgres.Open(cfg.dsn()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	//dbConn = dbConn.Debug()
	if err := migrate(dbConn); err != nil {
		return nil, err
	}
	if err := seed(dbConn); err != nil {
		return nil, err
	}
	logger.Info("db connection established")
	return dbConn, nil
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Application{},
		&Protection{},
		&Upstream{},
		&Ingress{},
		&Port{},
		&StateVersion{},
		&CrsVersion{},
		&CrsProfile{},
		&CrsRuleSet{},
	); err != nil {
		return err
	}
	migrated = true // mark as migrated
	return nil
}

func seed(db *gorm.DB) error {

	if err := db.FirstOrCreate(&StateVersion{TypeId: 1}).Error; err != nil {
		return err
	}
	if err := seedSQLTriggers(db); err != nil {
		return err
	}
	if err := seedCrsProfiles(db); err != nil {
		return err
	}
	seeded = true // mark as seeded
	return nil
}

func seedCrsProfiles(db *gorm.DB) error {

	crsRepo := NewCrsRepository(db, nil)
	profiles, err := crsRepo.GetProfileByName(DefaultCRSProfileName)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		rules, err := modsec.CRSRuleSet()
		if err != nil {
			return err
		}
		for crsName, crsContent := range rules {
			if err := crsRepo.CreateCrsProfile(
				&CrsProfile{
					Name:           DefaultCRSProfileName,
					CrsFileName:    crsName,
					CrsFileContent: crsContent,
				}); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedSQLTriggers(db *gorm.DB) error {
	rawSQL, err := sql.Triggers()
	if err != nil {
		return err
	}
	return db.Exec(rawSQL).Error
}

func db() *gorm.DB {
	if dbConn == nil {
		logger.Error("database connection not initialized, you must call NewDb(dbCfg) first")
	}
	if !migrated {
		if err := migrate(dbConn); err != nil {
			logger.Error("failed to migrate database", zap.Error(err))
		}
	}
	if !seeded {
		if err := seed(dbConn); err != nil {
			logger.Error("failed to seed database", zap.Error(err))
		}
	}
	return dbConn
}
