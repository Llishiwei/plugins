package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	PluginBridge    = "bridge"
	PluginHostLocal = "host-local"

	defaultDataDir = "/var/lib/cni/networks"

	BridgeDBName    = "bridge.db"
	HostLocalDBName = "host-local.db"
)

var db *gorm.DB

type BaseModel struct {
	ID        uint   `gorm:"primarykey"`
	Namespace string `gorm:"column:namespace"`
	Name      string `gorm:"column:name"`
	Deleted   bool   `gorm:"column:deleted"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// host-local reserved IP
type ReservedIP struct {
	IPv4 string `gorm:"column:ipv4"`
	IPv6 string `gorm:"column:ipv6"`
	BaseModel
}

func GetReservedIP(podNS, podName string) (ip ReservedIP, err error) {
	err = db.Take(&ip, "namespace = ? and name = ?", podNS, podName).Error
	return ip, err
}

func ReserveIP(ip *ReservedIP) error {
	return db.Save(ip).Error
}

func PurgeExpiredIPs(days int) error {
	end := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	return db.Delete(&ReservedIP{}, "deleted = ? and updated_at < ?", true, end).Error
}

// bridge reserved MAC
type ReservedMAC struct {
	MAC string `gorm:"column:mac"`
	BaseModel
}

func GetReservedMAC(podNS, podName string) (mac ReservedMAC, err error) {
	err = db.Take(&mac, "namespace = ? and name = ?", podNS, podName).Error
	return mac, err
}

func ReserveMAC(mac *ReservedMAC) error {
	return db.Save(mac).Error
}

func PurgeExpiredMACs(days int) error {
	end := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	return db.Delete(&ReservedMAC{}, "deleted = ? and updated_at < ?", true, end).Error
}

func IsNotFoundErr(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func ensureDataDir(network, dataDir string) (string, error) {
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	dir := filepath.Join(dataDir, network)
	err := os.MkdirAll(dir, 0755)
	return dir, err
}

func OpenDB(network, dataDir, pluginName string) error {
	dbName := ""
	switch pluginName {
	case PluginBridge:
		dbName = BridgeDBName
	case PluginHostLocal:
		dbName = HostLocalDBName
	default:
		return fmt.Errorf("not support plugin %s", pluginName)
	}
	dir, err := ensureDataDir(network, dataDir)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(dir, dbName)
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	switch pluginName {
	case PluginBridge:
		db.AutoMigrate(&ReservedMAC{})
	case PluginHostLocal:
		db.AutoMigrate(&ReservedIP{})
	}
	return nil
}

func CloseDB() error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
