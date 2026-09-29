package database

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"smallgo/server/logger"

	sqlmysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// coreModels are the framework's own tables, always migrated on startup.
var coreModels = []interface{}{
	&User{},
	&SystemConfig{},
	&UpgradeRecord{},
	&SecurityQuestion{},
	&UserSession{},
	&AuditLog{},
}

// appModels holds models contributed by registered apps via RegisterModels.
// Apps add to this slice from init(); AutoMigrate iterates both coreModels
// and appModels together so an app's schema lands on the same startup pass
// as the framework's own tables.
var appModels []interface{}

// RegisterModels adds GORM-managed models to the migration roster. Call
// from an app's init(); it is safe to call multiple times across packages.
//
//	func init() {
//	    database.RegisterModels(&Note{}, &Tag{})
//	}
func RegisterModels(models ...interface{}) {
	appModels = append(appModels, models...)
}

func InitDB(dbPath string) (*gorm.DB, error) {
	// Keep GORM quiet about expected "record not found" lookups (used heavily
	// for upsert-style config init) while still surfacing slow queries/errors.
	gormLog := gormlogger.New(log.New(logger.NewWriter("WARN"), "", 0), gormlogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
	})

	// Production Docker deployments set DB_DSN and use MySQL. Keeping the
	// SQLite fallback makes the existing unit tests and quick local `make dev`
	// workflow self-contained without making SQLite the production default.
	var db *gorm.DB
	var err error
	dsn := strings.TrimSpace(os.Getenv("DB_DSN"))
	if dsn == "" {
		dsn = mysqlDSNFromEnv()
	}
	if dsn != "" {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLog})
	} else {
		// Embed pragmas in the DSN so every pooled connection inherits them,
		// not only the connection that happened to run the PRAGMA exec below.
		dsn := dbPath +
			"?_pragma=journal_mode(WAL)" +
			"&_pragma=busy_timeout(5000)" +
			"&_pragma=synchronous(NORMAL)" +
			"&_pragma=foreign_keys(ON)"
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormLog})
	}
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if dsn == "" {
		// SQLite serializes writes through a single connection to avoid
		// SQLITE_BUSY under concurrent goroutines.
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	} else {
		// MySQL is the persistent production store. A small pool is enough for
		// a personal NAS deployment and avoids creating idle connections during
		// long periods with no reminders.
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	}

	return db, nil
}

// mysqlDSNFromEnv supports the simple external-MySQL variables used by the
// Docker Compose deployment. DB_DSN remains the more advanced override.
func mysqlDSNFromEnv() string {
	host := strings.TrimSpace(os.Getenv("MYSQL_HOST"))
	user := strings.TrimSpace(os.Getenv("MYSQL_USER"))
	dbName := strings.TrimSpace(os.Getenv("MYSQL_DB_NAME"))
	if dbName == "" {
		dbName = strings.TrimSpace(os.Getenv("MYSQL_DATABASE"))
	}
	if host == "" || user == "" || dbName == "" {
		return ""
	}
	port := strings.TrimSpace(os.Getenv("MYSQL_PORT"))
	if port == "" {
		port = "3306"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return ""
	}
	conf := sqlmysql.Config{
		User:      user,
		Passwd:    os.Getenv("MYSQL_PASSWORD"),
		Net:       "tcp",
		Addr:      net.JoinHostPort(host, port),
		DBName:    dbName,
		ParseTime: true,
		Loc:       time.Local,
		Params:    map[string]string{"charset": "utf8mb4"},
	}
	return conf.FormatDSN()
}

// AutoMigrate runs GORM's AutoMigrate across all framework models and any
// additional models contributed by apps via RegisterModels.
func AutoMigrate(db *gorm.DB) error {
	models := make([]interface{}, 0, len(coreModels)+len(appModels))
	models = append(models, coreModels...)
	models = append(models, appModels...)
	return db.AutoMigrate(models...)
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
